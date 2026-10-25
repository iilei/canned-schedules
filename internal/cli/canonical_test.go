package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/iilei/canned-schedules/internal/schedule"
)

// TestCanonicalRoundTrip writes every golden fixture with output in its canonical form and in its text form,
// evaluates both again with the fixture's args, and requires the same output: digest, week, transitions, annotations
// and the warning codes (positions move, because prose is gone).
func TestCanonicalRoundTrip(t *testing.T) {
	t.Parallel()

	for _, group := range []string{"valid", "warn"} {
		cases, err := os.ReadDir(filepath.Join(goldenDir, group))
		if err != nil {
			t.Fatal(err)
		}

		for _, c := range cases {
			name := group + "/" + c.Name()
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				dir := filepath.Join(goldenDir, name)
				src := []byte(readFile(t, filepath.Join(dir, "input.md")))
				args := splitLines(readOptional(t, filepath.Join(dir, "args")))

				out, _ := parseAndRun(args, src)
				if out == nil {
					t.Fatal("the fixture must evaluate")
				}

				opts, _ := Parse(args)

				canon, err := Canonical(&opts, src)
				if err != nil {
					t.Fatal(err)
				}

				text, err := Text(&opts, out, src, false)
				if err != nil {
					t.Fatal(err)
				}

				for form, again := range map[string][]byte{"canonical": canon, "text": text} {
					got, diags := parseAndRun(args, again)
					if got == nil {
						t.Fatalf("the %s form must evaluate: %v\n%s", form, diags, again)
					}

					if a, b := reduced(t, out), reduced(t, got); !reflect.DeepEqual(a, b) {
						t.Errorf("%s form changes the output:\n got: %s\nwant: %s\n%s", form, b, a, again)
					}
				}

				// The canonical form is a fixed point.
				if twice, err := Canonical(&opts, canon); err != nil || !bytes.Equal(twice, canon) {
					t.Errorf("canonical form is not stable:\n%s\nthen:\n%s", canon, twice)
				}
			})
		}
	}
}

// reduced is the output as JSON with each warning reduced to its code.
func reduced(t *testing.T, out *schedule.Output) string {
	t.Helper()

	o := *out
	o.Warnings = nil

	for _, w := range out.Warnings {
		o.Warnings = append(o.Warnings, schedule.Warning{Code: w.Code})
	}

	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}
