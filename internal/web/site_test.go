package web

import (
	"slices"
	"strings"
	"testing"
)

// gitlabSite is what the companion's playground build would write for a GitLab catalog.
const gitlabSite = `{
  "name": "Acme schedules",
  "args": ["--error-unless-alert-pattern", "(?i)NOTE|TIP", "--require-base", "true"],
  "catalog": {"dir": "schedules", "names": ["office-hours", "on-call"]},
  "propose": {
    "create": "https://gitlab.example.com/acme/catalog/-/new/main?file_name={path}&content={content}",
    "update": "https://gitlab.example.com/acme/catalog/-/edit/main/{path}?content={content}",
    "issue": "https://gitlab.example.com/acme/catalog/-/issues/new?issue[title]={name}&issue[description]={content}"
  }
}`

func TestCheckSite(t *testing.T) {
	t.Parallel()

	res := CheckSite([]byte(gitlabSite))
	if len(res.Errors) != 0 || res.Site == nil {
		t.Fatalf("errors %q", res.Errors)
	}

	s := res.Site
	if s.Name != "Acme schedules" || len(s.Args) != 4 ||
		!slices.Equal(s.Catalog.Names, []string{"office-hours", "on-call"}) {
		t.Errorf("site %+v", s)
	}

	if s.Propose == nil || s.Propose.MaxLength != defaultMaxLength || s.Propose.Update == nil ||
		!s.Propose.Update.Prefill || !strings.Contains(s.Propose.Update.URL, "/-/edit/main/{path}") {
		t.Errorf("propose %+v", s.Propose)
	}

	if !strings.Contains(compact(res.Encode()), `"errors":[]`) {
		t.Errorf("encoded %s", res.Encode())
	}
}

// TestCheckSiteDefaults: an empty site is valid and gets the defaults, with lists instead of nulls for JavaScript.
func TestCheckSiteDefaults(t *testing.T) {
	t.Parallel()

	res := CheckSite([]byte(`{}`))
	if len(res.Errors) != 0 || res.Site == nil || res.Site.Catalog.Dir != "schedules" || res.Site.Propose != nil {
		t.Fatalf("%+v", res)
	}

	if enc := compact(res.Encode()); !strings.Contains(enc, `"args":[]`) || !strings.Contains(enc, `"names":[]`) {
		t.Errorf("encoded %s", enc)
	}

	// GitHub's edit page takes no content: copy, then open.
	res = CheckSite(
		[]byte(`{"propose": {"update": {"url": "https://github.com/o/r/edit/main/{path}", "prefill": false}}}`),
	)
	if len(res.Errors) != 0 || res.Site.Propose.Update.Prefill {
		t.Errorf("%+v", res)
	}
}

func TestCheckSiteRefuses(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct{ json, want string }{
		"not JSON":         {`{`, "not JSON"},
		"unknown key":      {`{"colour": 1}`, "colour"},
		"bad name":         {`{"name": "Acme!"}`, "name"},
		"bad catalog name": {`{"catalog": {"names": ["Office Hours"]}}`, "catalog.names"},
		"dot dot":          {`{"catalog": {"dir": "a/../b"}}`, "catalog.dir"},
		"http":             {`{"propose": {"issue": "http://x.example/{content}"}}`, "propose.issue"},
		"no content":       {`{"propose": {"create": "https://x.example/new?file={path}"}}`, "needs {content}"},
		"no path":          {`{"propose": {"update": "https://x.example/edit?c={content}"}}`, "needs {path}"},
		"unknown ph":       {`{"propose": {"issue": "https://x.example/?t={title}&b={content}"}}`, "unknown placeholder {title}"},
		"user info":        {`{"propose": {"issue": "https://me@x.example/?b={content}"}}`, "user info"},
		"fragment":         {`{"propose": {"issue": "https://x.example/#{content}"}}`, "fragment"},
		"page flag":        {`{"args": ["--sample-interval", "15"]}`, "control"},
		"output flag":      {`{"args": ["--svg-output-file", "x.svg"]}`, "from site.json: args take"},
		"bad value":        {`{"args": ["--require-base", "maybe"]}`, "args:"},
		"short max":        {`{"propose": {"max-length": 10}}`, "max-length"},
	} {
		res := CheckSite([]byte(c.json))
		if res.Site != nil || !strings.Contains(strings.Join(res.Errors, "\n"), c.want) {
			t.Errorf("%s: site %v, errors %q; want an error with %q", name, res.Site, res.Errors, c.want)
		}
	}
}

func TestEvaluateFile(t *testing.T) {
	t.Parallel()

	ok := Evaluate([]byte("# Night Shift\n\n> [!SCHEDULE-STATE] Medium\n> - `22:00` to `06:00` MON-FRI\n"), nil, nil)
	if ok.File != "night-shift.canned-schedule.md" {
		t.Errorf("file %q", ok.File)
	}

	if bad := Evaluate([]byte("# Night Shift!\n"), nil, nil); bad.File != "" {
		t.Errorf("an invalid document gets no file: %q", bad.File)
	}
}

// compact drops the encoder's indentation, so tests can look for fragments.
func compact(s string) string { return strings.Join(strings.Fields(s), "") }
