package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	presetDir = "../../web/presets"
	// officeHours is copied from the GitLab issue template at build time (mise run web:build), not kept twice.
	officeHours    = "office-hours"
	gitlabTemplate = "../../.gitlab/issue_templates/canned_schedule_request.md"
)

// controlFlags maps a preset's options (the page's controls) to the flags the page sends for them.
var controlFlags = map[string]func(string) []string{
	"symbols":         func(v string) []string { return []string{"--ascii-symbols", v} },
	"sample-interval": func(v string) []string { return []string{"--sample-interval", v} },
	"downsample":      func(v string) []string { return []string{"--preview-downsample=" + v} },
}

type preset struct {
	Options  map[string]string `json:"options"` // Values for the page's controls.
	ID       string            `json:"id"`
	Label    string            `json:"label"`
	Category string            `json:"category"` // The dropdown's group.
	Args     bool              `json:"args"`     // presets/<id>.args holds flags for the checker.
	Hidden   bool              `json:"hidden"`   // Not in the dropdown; opened by link only (?preset=<id>).
}

// argLines reads an args file as the page does: one argument per line, blank lines and # comments skipped.
func argLines(text string) []string {
	var args []string

	for l := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, "#") {
			args = append(args, l)
		}
	}

	return args
}

// presetFlags returns the preset's flags (from its args file) and the flags of its options.
func presetFlags(t *testing.T, p preset) ([]string, []string) {
	t.Helper()

	var link, page []string

	if p.Args {
		raw, err := os.ReadFile(filepath.Join(presetDir, p.ID+".args"))
		if err != nil {
			t.Errorf("preset %q: %v", p.ID, err)
		}

		link = argLines(string(raw))
	}

	for k, v := range p.Options {
		f := controlFlags[k]
		if f == nil {
			t.Errorf("preset %q: unknown option %q", p.ID, k)

			continue
		}

		page = append(page, f(v)...)
	}

	return link, page
}

// TestPresets keeps the page's presets valid: every listed preset evaluates without diagnostics, and the index and
// the files match.
func TestPresets(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(presetDir, "index.json"))
	if err != nil {
		t.Fatal(err)
	}

	var list []preset
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}

	listed := map[string]bool{}

	for _, p := range list {
		if listed[p.ID] || p.Label == "" || p.Category == "" {
			t.Errorf("preset %q: ids must be unique, labels and categories set", p.ID)
		}

		listed[p.ID] = true

		path := filepath.Join(presetDir, p.ID+".md")
		if p.ID == officeHours {
			path = gitlabTemplate
		}

		src, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("preset %q: %v", p.ID, err)

			continue
		}

		link, page := presetFlags(t, p)
		if res := Evaluate(src, link, page); res.Errors+res.Warnings != 0 {
			t.Errorf("preset %q must evaluate cleanly, got report %s", p.ID, res.Report)
		}
	}

	if !listed[officeHours] {
		t.Errorf("the %q preset (the issue template) must be listed", officeHours)
	}

	files, _ := filepath.Glob(filepath.Join(presetDir, "*.md"))
	for _, f := range files {
		if id := strings.TrimSuffix(filepath.Base(f), ".md"); !listed[id] || id == officeHours {
			t.Errorf("%s is not listed in index.json (or duplicates the issue template)", f)
		}
	}

	withArgs := map[string]bool{}
	for _, p := range list {
		withArgs[p.ID] = p.Args
	}

	argFiles, _ := filepath.Glob(filepath.Join(presetDir, "*.args"))
	for _, f := range argFiles {
		if id := strings.TrimSuffix(filepath.Base(f), ".args"); !withArgs[id] {
			t.Errorf("%s belongs to no preset with \"args\": true", f)
		}
	}
}
