package web

import (
	"strings"
	"testing"
)

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

// TestCheckSiteForThePage: the page gets lists rather than nulls.
func TestCheckSiteForThePage(t *testing.T) {
	t.Parallel()

	res := CheckSite([]byte(`{}`))
	if enc := strings.Join(strings.Fields(res.Encode()), ""); !strings.Contains(enc, `"errors":[]`) ||
		!strings.Contains(enc, `"args":[]`) || !strings.Contains(enc, `"names":[]`) {
		t.Errorf("encoded %s", enc)
	}

	if res := CheckSite([]byte(`{`)); res.Site != nil || len(res.Errors) != 1 {
		t.Errorf("%+v", res)
	}
}

// TestEvaluateSiteWins: on a self-hosted playground the site's flags decide. A link that sets one of them is refused
// (and its flags left out), and the page's controls can't loosen them either: the site's flags come last.
func TestEvaluateSiteWins(t *testing.T) {
	t.Parallel()

	src := []byte("# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `06:05` to `22:00` MON-FRI\n")
	site := []string{"--sample-interval", "5", "--require-base", "true"}

	res := EvaluateSite(src, site, []string{"--require-base", "false"}, nil, false)
	if res.Errors == 0 || !strings.Contains(res.Report, "set by this playground") {
		t.Errorf("a link must not change a site flag: %s", res.Report)
	}

	// The page's sample-interval control says 1, the site 5: 06:05 is fine with 5, but the site wants a BASE.
	res = EvaluateSite(src, site, nil, []string{"--sample-interval", "1"}, false)
	if !strings.Contains(res.Report, "missing-base") && !strings.Contains(res.Report, "BASE") {
		t.Errorf("the site's --require-base must hold: %s", res.Report)
	}

	strict := []string{"--sample-interval", "15"}
	if res := EvaluateSite(
		src,
		strict,
		nil,
		[]string{"--sample-interval", "5"},
		false,
	); !strings.Contains(
		res.Report,
		"sample-interval",
	) {
		t.Errorf("the page's control must not loosen the site's interval: %s", res.Report)
	}

	if res := EvaluateSite(src, nil, nil, []string{"--sample-interval", "5"}, false); res.Errors != 0 {
		t.Errorf("without a site the page's control decides: %s", res.Report)
	}
}

// TestPlaygroundGenerator: a team's copy names itself in the SVG it renders, so a downloaded picture doesn't claim to
// come from the public preflight page.
func TestPlaygroundGenerator(t *testing.T) {
	t.Parallel()

	src := []byte("# Office\n\n> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n")

	res := EvaluateSite(src, nil, nil, nil, true)
	if !strings.Contains(res.SVG, `generator="`+Playground) {
		t.Errorf("the playground's SVG must name it as its generator:\n%.400s", res.SVG)
	}

	if res := Evaluate(src, nil, nil); !strings.Contains(res.SVG, `generator="`+PreflightPage) {
		t.Errorf("the preflight page's SVG must name it as its generator:\n%.400s", res.SVG)
	}
}
