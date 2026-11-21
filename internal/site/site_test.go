package site

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
    "issue": "https://gitlab.example.com/acme/catalog/-/issues/new?issue[title]={name}&issue[description]={content}",
    "fork": "https://gitlab.example.com/acme/catalog/-/forks/new"
  }
}`

// result is what check returns: Check's two values as one, for table tests.
type result struct {
	Site   *Site
	Errors []string
}

func TestCheckSite(t *testing.T) {
	t.Parallel()

	res := check([]byte(gitlabSite))
	if len(res.Errors) != 0 || res.Site == nil {
		t.Fatalf("errors %q", res.Errors)
	}

	s := res.Site
	if s.Name != "Acme schedules" || len(s.Args) != 4 ||
		!slices.Equal(s.Catalog.Names, []string{"office-hours", "on-call"}) {
		t.Errorf("site %+v", s)
	}

	if s.Propose == nil || s.Propose.MaxLength != defaultMaxLength || s.Propose.Update == nil ||
		!s.Propose.Update.Prefill || !strings.Contains(s.Propose.Update.URL, "/-/edit/main/{path}") ||
		s.Propose.Fork != "https://gitlab.example.com/acme/catalog/-/forks/new" {
		t.Errorf("propose %+v", s.Propose)
	}
}

// TestCheckSiteDefaults: an empty site is valid and gets the defaults, with lists instead of nulls for JavaScript.
func TestCheckSiteDefaults(t *testing.T) {
	t.Parallel()

	res := check([]byte(`{}`))
	if len(res.Errors) != 0 || res.Site == nil || res.Site.Catalog.Dir != "schedules" || res.Site.Propose != nil {
		t.Fatalf("%+v", res)
	}

	if res.Site.Args == nil || res.Site.Catalog.Names == nil {
		t.Errorf("lists, not nil: %+v", res.Site)
	}

	// GitHub's edit page takes no content: copy, then open.
	res = check(
		[]byte(`{"propose": {"update": {"url": "https://github.com/o/r/edit/main/{path}", "prefill": false}}}`),
	)
	if len(res.Errors) != 0 || res.Site.Propose.Update.Prefill {
		t.Errorf("%+v", res)
	}
}

// TestCheckSiteSampleInterval: a site may set the sample interval, which changes what is valid; the page locks its
// control to the site's value.
func TestCheckSiteSampleInterval(t *testing.T) {
	t.Parallel()

	if res := check([]byte(`{"args": ["--sample-interval", "15"]}`)); res.Site == nil || len(res.Errors) != 0 {
		t.Errorf("%+v", res)
	}

	if res := check([]byte(`{"args": ["--sample-interval", "7"]}`)); res.Site != nil {
		t.Error("an interval that doesn't divide 60 must be refused")
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
		"preview flag":     {`{"args": ["--ascii-symbols", "braille"]}`, "can't come from site.json"},
		"output flag":      {`{"args": ["--svg-output-file", "x.svg"]}`, "from site.json: its args take"},
		"bad value":        {`{"args": ["--require-base", "maybe"]}`, "args:"},
		"short max":        {`{"propose": {"max-length": 10}}`, "max-length"},
		"fork placeholder": {`{"propose": {"fork": "https://x.example/{path}"}}`, "propose.fork"},
	} {
		res := check([]byte(c.json))
		if res.Site != nil || !strings.Contains(strings.Join(res.Errors, "\n"), c.want) {
			t.Errorf("%s: site %v, errors %q; want an error with %q", name, res.Site, res.Errors, c.want)
		}
	}
}

func check(raw []byte) result {
	s, errs := Check(raw)

	return result{s, errs}
}
