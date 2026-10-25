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
