package web

import (
	"os"
	"strings"
	"testing"
)

func TestEvaluateValid(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("../../.gitlab/issue_templates/canned_schedule_request.md")
	if err != nil {
		t.Fatal(err)
	}

	res := Evaluate(src, nil, []string{"--svg-theme", "dark"})
	if res.Errors != 0 || res.JSON == "" || res.ASCII == "" || res.SphinxCSS == "" ||
		!strings.Contains(res.MyST, ":::{admonition}") {
		t.Fatalf("result = %+v", res)
	}

	if !strings.Contains(res.SVG, "color-scheme: dark") {
		t.Error("the SVG must follow --svg-theme")
	}

	// The page's SVG always carries the schedule in canonical form.
	if !strings.Contains(res.SVG, "&gt; [!SCHEDULE-STATE] Medium") ||
		!strings.Contains(
			res.SVG,
			`generator="canned-schedules preflight page`,
		) || strings.Contains(res.SVG, "digest=") {
		t.Error("the page's SVG must embed the source")
	}

	// The name first; every range highlight points at a range item and covers part of the week.
	if len(res.Highlights) < 2 || res.Highlights[0].Kind != "name" {
		t.Fatalf("highlights = %+v", res.Highlights)
	}

	lines := strings.Split(string(src), "\n")
	for _, h := range res.Highlights {
		if h.Kind != "range" {
			continue
		}

		if h.Line < 1 || !strings.HasPrefix(strings.TrimSpace(lines[h.Line-1]), "> - `") ||
			len(h.Rects)+len(h.Shadowed) == 0 {
			t.Errorf("highlight %+v doesn't point at a range item", h)
		}
	}

	if !strings.Contains(res.GitHubHTML, "<h1>Office Hours</h1>") || !strings.Contains(res.SphinxHTML, "admonition") {
		t.Error("both previews must be rendered")
	}

	if !strings.Contains(res.Encode(), `"report-markdown"`) {
		t.Error("Encode must produce the JSON the page reads")
	}
}

func TestEvaluateInvalid(t *testing.T) {
	t.Parallel()

	res := Evaluate([]byte("# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `10:00` to `24:00` SAT\n"), nil, nil)
	if res.Errors != 1 || res.JSON != "" || res.SVG != "" || res.MyST != "" || res.Highlights != nil {
		t.Fatalf("an invalid document has no output, got %+v", res)
	}

	if res.GitHubHTML == "" || !strings.Contains(res.Report, `"location": "schedule.md:4:17"`) {
		t.Errorf("previews and the report still render: %+v", res)
	}

	if !strings.Contains(res.ReportMarkdown, "#### error `invalid-time`") {
		t.Errorf("Markdown report = %q", res.ReportMarkdown)
	}
}

func TestEvaluateFlagError(t *testing.T) {
	t.Parallel()

	res := Evaluate([]byte("# Plan\n"), nil, []string{"--svg-theme", "sepia"})
	if res.Errors != 1 || !strings.Contains(res.Report, "invalid-flag") {
		t.Errorf("result = %+v", res)
	}
}

func TestSphinxKit(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("../../.gitlab/issue_templates/canned_schedule_request.md")
	if err != nil {
		t.Fatal(err)
	}

	zip, err := SphinxKit(src, nil, []string{"--svg-theme", "dark"})
	if err != nil || len(zip) < 1000 || string(zip[:2]) != "PK" {
		t.Fatalf("kit: %d bytes, err %v", len(zip), err)
	}

	if _, err := SphinxKit(
		[]byte("# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `10:00` to `24:00` SAT\n"),
		nil,
		nil,
	); err == nil {
		t.Error("an invalid schedule must not produce a kit")
	}
}

func TestEvaluateStates(t *testing.T) {
	t.Parallel()

	src := []byte("# Rota\n\n> [!SCHEDULE-STATE] Primary\n> - `08:00` to `16:00` MON-FRI\n")

	if got := Evaluate(src, nil, nil).States; len(got) != 4 || got[0].ID != "Off" {
		t.Errorf("without overrides, States = %v, want the built-in states", got)
	}

	set := func(value string) []string { return []string{"--set-config-at", "states", "--set-config-value", value} }

	res := Evaluate(
		src,
		nil,
		set(`[{"id":"Off","display-name":"Off"},{"id":"Primary","display-name":"Primary on-call"}]`),
	)
	if res.Errors != 0 || len(res.States) != 2 || res.States[1].DisplayName != "Primary on-call" {
		t.Errorf("with a states override: errors %d, States = %v", res.Errors, res.States)
	}

	res = Evaluate(src, nil, set(`[{"id":"Off","display-name":"Off"},{"id":"OFF","display-name":"Off again"}]`))
	if res.States != nil || !strings.Contains(res.Report, `"code": "config-duplicate-state"`) {
		t.Errorf("an invalid override: States = %v, report %s", res.States, res.Report)
	}
}
