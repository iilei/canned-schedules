package cli

import (
	"strings"
	"testing"
)

const (
	githubTemplate = "../../.github/ISSUE_TEMPLATE/canned_schedule_request.md"
	gitlabTemplate = "../../.gitlab/issue_templates/canned_schedule_request.md"
)

// TestIssueTemplates keeps the issue templates valid and identical. Contributors start from them, so a template
// that fails validation would greet every new request with errors.
func TestIssueTemplates(t *testing.T) {
	t.Parallel()

	github := readFile(t, githubTemplate)
	gitlab := readFile(t, gitlabTemplate)

	body, ok := stripFrontmatter(github)
	if !ok {
		t.Fatal("the GitHub template needs its frontmatter (name, about, title, labels)")
	}

	if body != gitlab {
		t.Error("the GitHub and GitLab templates must have the same body; GitHub only adds frontmatter")
	}

	out, diags := parseAndRun(nil, []byte(body))
	for _, d := range diags {
		t.Errorf("template: %s", d.Format("template"))
	}

	if out == nil {
		t.Fatal("the template must produce output")
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
