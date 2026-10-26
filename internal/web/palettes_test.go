package web

import (
	"encoding/json"
	"os"
	"testing"
)

const (
	paletteIndex = "../../web/palettes/index.json"
	// builtIn is the palette without styles: the config's own.
	builtIn = "built-in"
)

type palette struct {
	ID     string          `json:"id"`
	Label  string          `json:"label"`
	Kind   string          `json:"kind"`
	Styles json.RawMessage `json:"styles"`
}

// TestPalettes keeps the page's colour palettes valid: each one, applied as the page applies it, evaluates the issue
// template without diagnostics, so it passes the config schema and has a style for each built-in state (no round
// robin).
func TestPalettes(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(paletteIndex)
	if err != nil {
		t.Fatal(err)
	}

	var list []palette
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}

	src, err := os.ReadFile(gitlabTemplate)
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}

	for _, p := range list {
		if seen[p.ID] || p.Label == "" || (p.Kind != "sequential" && p.Kind != "categorical") {
			t.Errorf("palette %q: ids must be unique, labels set and the kind sequential or categorical", p.ID)
		}

		seen[p.ID] = true

		if (p.ID == builtIn) != (p.Styles == nil) {
			t.Errorf("palette %q: only %q comes without styles", p.ID, builtIn)

			continue
		}

		if p.Styles == nil {
			continue
		}

		args := []string{"--set-config-at", "representation.styles.states", "--set-config-value", string(p.Styles)}
		if res := Evaluate(src, args); res.Errors+res.Warnings != 0 {
			t.Errorf("palette %q must apply cleanly, got report %s", p.ID, res.Report)
		}
	}

	if len(list) == 0 || list[0].ID != builtIn {
		t.Errorf("the first palette must be %q, the page's default", builtIn)
	}
}
