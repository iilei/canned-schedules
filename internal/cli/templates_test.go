package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	githubTemplates = "../../.github/ISSUE_TEMPLATE/"
	gitlabTemplates = "../../.gitlab/issue_templates/"
	gitlabTemplate  = gitlabTemplates + "canned_schedule_request.md"
	docsPreview     = "../../docs/office-hours.svg"
	builtInConfig   = "../../schema/config.json"

	// gitlabLabel sets the label on GitLab, whose templates have no frontmatter. GitLab removes the line when it
	// stores the issue.
	gitlabLabel = "/label ~Schedule\n"
	syntaxLink  = "[Syntax reference](https://github.com/iilei/canned-schedules#syntax)"
)

// TestIssueTemplates keeps the issue templates valid and in step. Contributors learn the syntax from them, so a template
// that fails validation would greet every new request with errors.
func TestIssueTemplates(t *testing.T) {
	t.Parallel()

	statesHint := "<!-- States you can use: " + strings.Join(builtInStates(t), ", ") + " -->"

	for _, name := range []string{
		"canned_schedule_request.md", "canned_schedule_night_shift.md", "canned_schedule_friday_peak.md",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body, ok := stripFrontmatter(readFile(t, githubTemplates+name))
			if !ok {
				t.Fatal("the GitHub template needs its frontmatter (name, about, title, labels)")
			}

			if readFile(t, gitlabTemplates+name) != body+gitlabLabel {
				t.Error("the GitLab template must be the GitHub body followed by the label line")
			}

			if !strings.Contains(body, statesHint) {
				t.Errorf("the template must name the built-in states: %s", statesHint)
			}

			if !strings.Contains(body, syntaxLink) {
				t.Errorf("the template must link to the syntax: %s", syntaxLink)
			}

			out, diags := parseAndRun(nil, []byte(body))
			for _, d := range diags {
				t.Errorf("template: %s", d.Format("template"))
			}

			if out == nil {
				t.Fatal("the template must produce output")
			}
		})
	}
}

func builtInStates(t *testing.T) []string {
	t.Helper()

	var config struct {
		States []struct {
			ID string `json:"id"`
		} `json:"states"`
	}

	if err := json.Unmarshal([]byte(readFile(t, builtInConfig)), &config); err != nil {
		t.Fatal(err)
	}

	ids := make([]string, 0, len(config.States))
	for _, s := range config.States {
		ids = append(ids, s.ID)
	}

	return ids
}

// TestDocsPreview keeps the README image in sync with the issue template and the SVG renderer. Regenerate with:
// go run ./cmd/canned-schedules --svg-output-file docs/office-hours.svg \
// .gitlab/issue_templates/canned_schedule_request.md.
func TestDocsPreview(t *testing.T) {
	t.Parallel()

	opts, diags := Parse(nil)
	if diags.HasErrors() {
		t.Fatal(diags)
	}

	out, _ := Run(&opts, []byte(readFile(t, gitlabTemplate)))
	if out == nil {
		t.Fatal("the template must evaluate")
	}

	svg, err := SVG(&opts, out, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	if string(svg) != readFile(t, docsPreview) {
		t.Errorf("%s is stale; regenerate it (see the test comment)", docsPreview)
	}
}

// stripFrontmatter removes GitHub's issue template frontmatter. GitHub drops it from the issue body.
func stripFrontmatter(s string) (string, bool) {
	rest, ok := strings.CutPrefix(s, "---\n")
	if !ok {
		return s, false
	}

	_, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return s, false
	}

	return strings.TrimPrefix(body, "\n"), true
}
