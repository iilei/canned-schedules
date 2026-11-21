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

	syntaxLink = "[Syntax reference](https://github.com/iilei/canned-schedules#syntax)"
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

			front, body, ok := splitFrontmatter(readFile(t, githubTemplates+name))
			if !ok {
				t.Fatal("the GitHub template needs its frontmatter (name, about, title, labels)")
			}

			checkFrontmatter(t, front)

			// No label line: CI sets the label once the schedule is valid (CLAUDE.md, "Issue trigger and label").
			if readFile(t, gitlabTemplates+name) != body {
				t.Error("the GitLab template must be the GitHub body")
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

// splitFrontmatter separates GitHub's issue template frontmatter from the body. GitHub drops it from the issue body.
func splitFrontmatter(s string) (string, string, bool) {
	rest, ok := strings.CutPrefix(s, "---\n")
	if !ok {
		return "", s, false
	}

	front, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return "", s, false
	}

	return front, strings.TrimPrefix(body, "\n"), true
}

// checkFrontmatter catches frontmatter GitHub can't read. GitHub ignores a template whose YAML doesn't parse, without
// a word: the chooser jumps to a blank issue and the labels are never set. That happened with an unquoted
// "name: Schedule request: office hours". The repo has no YAML parser (it isn't needed at runtime), so this checks the
// flat "key: value" shape the templates use: a plain value must not contain ": " or " #" and must not start with a
// character YAML treats specially; quote it instead.
func checkFrontmatter(t *testing.T, front string) {
	t.Helper()

	for line := range strings.SplitSeq(front, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || key == "" || strings.ContainsAny(key, " \t\"'") {
			t.Errorf("frontmatter line %q: want key: value", line)

			continue
		}

		value = strings.TrimSpace(value)
		if value == "" || strings.ContainsRune(`"'[`, rune(value[0])) {
			continue
		}

		if strings.Contains(value, ": ") || strings.Contains(value, " #") ||
			strings.ContainsRune("&*!|>%@`{", rune(value[0])) ||
			strings.HasPrefix(value, "- ") || strings.HasPrefix(value, "? ") {
			t.Errorf("frontmatter line %q: quote the value, YAML would not read it as plain text", line)
		}
	}
}
