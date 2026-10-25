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

type preset struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Hidden bool   `json:"hidden"` // Not in the dropdown; opened by link only (?preset=<id>).
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
		if listed[p.ID] || p.Label == "" {
			t.Errorf("preset %q: ids must be unique and labels set", p.ID)
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

		if res := Evaluate(src, nil); res.Errors+res.Warnings != 0 {
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
}
