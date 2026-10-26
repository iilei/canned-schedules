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

	res := Evaluate(src, []string{"--svg-theme", "dark"})
	if res.Errors != 0 || res.JSON == "" || res.ASCII == "" || res.SphinxCSS == "" ||
		!strings.Contains(res.MyST, ":::{admonition}") {
		t.Fatalf("result = %+v", res)
	}

	if !strings.Contains(res.SVG, "color-scheme: dark") {
		t.Error("the SVG must follow --svg-theme")
	}

	// The page's SVG always carries the canonical request.
	if !strings.Contains(res.SVG, "&gt; [!SCHEDULE-STATE] Medium") ||
		!strings.Contains(
			res.SVG,
			`generator="canned-schedules preflight page`,
		) || strings.Contains(res.SVG, "digest=") {
		t.Error("the page's SVG must embed the source")
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

	res := Evaluate([]byte("# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `10:00` to `24:00` SAT\n"), nil)
	if res.Errors != 1 || res.JSON != "" || res.SVG != "" || res.MyST != "" {
		t.Fatalf("an invalid document has no output, got %+v", res)
	}

	if res.GitHubHTML == "" || !strings.Contains(res.Report, `"location": "request.md:4:17"`) {
		t.Errorf("previews and the report still render: %+v", res)
	}

	if !strings.Contains(res.ReportMarkdown, "#### error `invalid-time`") {
		t.Errorf("Markdown report = %q", res.ReportMarkdown)
	}
}

func TestEvaluateFlagError(t *testing.T) {
	t.Parallel()

	res := Evaluate([]byte("# Plan\n"), []string{"--svg-theme", "sepia"})
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

	zip, err := SphinxKit(src, []string{"--svg-theme", "dark"})
	if err != nil || len(zip) < 1000 || string(zip[:2]) != "PK" {
		t.Fatalf("kit: %d bytes, err %v", len(zip), err)
	}

	if _, err := SphinxKit(
		[]byte("# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `10:00` to `24:00` SAT\n"),
		nil,
	); err == nil {
		t.Error("an invalid request must not produce a kit")
	}
}

func TestEvaluateStates(t *testing.T) {
	t.Parallel()

	src := []byte("# Rota\n\n> [!SCHEDULE-STATE] Primary\n> - `08:00` to `16:00` MON-FRI\n")

	if got := Evaluate(src, nil).States; len(got) != 4 || got[0].ID != "Off" {
		t.Errorf("without overrides, States = %v, want the built-in states", got)
	}

	set := func(value string) []string { return []string{"--set-config-at", "states", "--set-config-value", value} }

	res := Evaluate(src, set(`[{"id":"Off","display-name":"Off"},{"id":"Primary","display-name":"Primary on-call"}]`))
	if res.Errors != 0 || len(res.States) != 2 || res.States[1].DisplayName != "Primary on-call" {
		t.Errorf("with a states override: errors %d, States = %v", res.Errors, res.States)
	}

	res = Evaluate(src, set(`[{"id":"Off","display-name":"Off"},{"id":"OFF","display-name":"Off again"}]`))
	if res.States != nil || !strings.Contains(res.Report, `"code": "config-duplicate-state"`) {
		t.Errorf("an invalid override: States = %v, report %s", res.States, res.Report)
	}
}
