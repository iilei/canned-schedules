package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func linkDiags(t *testing.T, res Result) []string {
	t.Helper()

	var r struct {
		Diagnostics []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(res.Report), &r); err != nil {
		t.Fatal(err)
	}

	var out []string
	for _, d := range r.Diagnostics {
		out = append(out, d.Code+": "+d.Message)
	}

	return out
}

func TestLinkArgs(t *testing.T) {
	t.Parallel()

	src := []byte("# Plan\n\n> [!TIP]\n> Prose.\n\n> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n")

	if res := Evaluate(src, nil, nil); res.Errors != 0 {
		t.Fatalf("without a link: %v", linkDiags(t, res))
	}

	// The CLI's checks apply to flags from a link, with or without an attached value.
	for _, link := range [][]string{
		{"--error-unless-alert-pattern", "NOTE"},
		{"--error-unless-alert-pattern=NOTE"},
		{"-require-base", "true"},
	} {
		if res := Evaluate(src, link, nil); res.Errors != 1 || res.JSON != "" {
			t.Errorf("%q: want one error, got %v", link, linkDiags(t, res))
		}
	}

	if res := Evaluate(src, []string{"--base", "Maximum"}, nil); res.Errors != 0 ||
		!strings.Contains(res.JSON, `"base": "Maximum"`) {
		t.Errorf("--base from a link: %v", linkDiags(t, res))
	}

	// Config overrides from the link come first: the page's own pair for the same path wins.
	link := []string{
		"--set-config-at",
		"states",
		"--set-config-value",
		`[{"id":"Off","display-name":"Off"},{"id":"Medium","display-name":"Linked"}]`,
	}
	if res := Evaluate(
		src,
		link,
		nil,
	); res.Errors != 0 || len(res.States) != 2 ||
		res.States[1].DisplayName != "Linked" {
		t.Errorf("states from a link: %+v %v", res.States, linkDiags(t, res))
	}

	page := []string{"--set-config-at", "states", "--set-config-value", `[{"id":"Medium","display-name":"Page"}]`}
	if res := Evaluate(src, link, page); len(res.States) != 1 || res.States[0].DisplayName != "Page" {
		t.Errorf("the page's override must win: %+v", res.States)
	}

	if res := Evaluate(src, []string{"--set-config-at", "states"}, nil); res.Errors != 1 {
		t.Errorf("an unpaired override: %v", linkDiags(t, res))
	}
}

func TestLinkArgsRefused(t *testing.T) {
	t.Parallel()

	src := []byte("# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n")

	for _, tc := range []struct {
		link []string
		want string
	}{
		{[]string{"--sample-interval", "15"}, "?sample-interval="},
		{[]string{"--preview-downsample=30"}, "?downsample="},
		{[]string{"--no-preview-downsample"}, "?downsample=1"},
		{[]string{"--ascii-symbols", "braille"}, "?symbols="},
		{[]string{"--svg-output-file", "x.svg"}, "can't come from a link"},
		{[]string{"--svg-theme", "dark"}, "can't come from a link"},
		{[]string{"--bogus"}, "--bogus can't come from a link"},
		{[]string{"-h"}, "-h can't come from a link"},
		{[]string{"schedule.md"}, "not a flag"},
		{[]string{"--"}, "not a flag"},
	} {
		res := Evaluate(src, tc.link, nil)

		got := linkDiags(t, res)
		if res.Errors != 1 || res.JSON != "" || len(got) != 1 || !strings.Contains(got[0], tc.want) ||
			!strings.HasPrefix(got[0], "invalid-flag: ") {
			t.Errorf("%q: want one invalid-flag with %q, got %v", tc.link, tc.want, got)
		}

		// The page's own flags still apply, so the config previews render.
		if len(res.States) == 0 || res.SphinxCSS == "" {
			t.Errorf("%q: the page's previews must still render", tc.link)
		}
	}
}
