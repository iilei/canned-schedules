package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/iilei/canned-schedules/internal/schedule"
)

const goldenDir = "../../testdata/golden"

// TestGolden runs every fixture in testdata/golden. The fixtures are the spec: never regenerate them from this code.
// Cases that reach a pipeline stage which isn't implemented yet are skipped, not failed.
func TestGolden(t *testing.T) {
	t.Parallel()

	for _, group := range []string{"valid", "warn", "error"} {
		cases, err := os.ReadDir(filepath.Join(goldenDir, group))
		if err != nil {
			t.Fatal(err)
		}

		for _, c := range cases {
			name := group + "/" + c.Name()
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				runGolden(t, filepath.Join(goldenDir, name))
			})
		}
	}
}

func runGolden(t *testing.T, dir string) {
	t.Helper()

	src := readFile(t, filepath.Join(dir, "input.md"))
	args := splitLines(readOptional(t, filepath.Join(dir, "args")))

	out, diags, err := Run(args, []byte(src))
	if errors.Is(err, schedule.ErrNotImplemented) {
		t.Skip("pipeline stage not implemented yet")
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotDiag := make([]string, 0, len(diags))
	for _, d := range diags {
		gotDiag = append(gotDiag, d.Golden())
	}

	wantDiag := splitLines(readOptional(t, filepath.Join(dir, "want.diag")))
	if !reflect.DeepEqual(gotDiag, wantDiag) {
		t.Errorf("diagnostics:\n got: %q\nwant: %q", gotDiag, wantDiag)

		for _, d := range diags {
			t.Logf("  %s", d.Format("input.md"))
		}
	}

	compareOutput(t, out, readOptional(t, filepath.Join(dir, "want.json")))
}

func compareOutput(t *testing.T, out *schedule.Output, wantJSON string) {
	t.Helper()

	if wantJSON == "" {
		if out != nil {
			t.Errorf("expected no output, got %+v", out)
		}

		return
	}

	if out == nil {
		t.Fatal("expected output, got none")
	}

	gotJSON, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}

	var got, want any
	if err := json.Unmarshal(gotJSON, &got); err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		pretty, _ := json.MarshalIndent(out, "", "  ")
		t.Errorf("output mismatch:\n got: %s\nwant: %s", pretty, wantJSON)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

func readOptional(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}

	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

// splitLines returns the non-empty lines of s; fixture args and diagnostics are one entry per line.
func splitLines(s string) []string {
	out := []string{}

	for line := range strings.SplitSeq(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}

	return out
}
