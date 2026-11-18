package cli

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/iilei/canned-schedules/internal/schedule"
)

const (
	constraintsDoc = "# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `08:00` to `18:00` MON-FRI\n"
	withBaseDoc    = "# Plan\n\n> [!SCHEDULE-BASE] Off\n\n> [!SCHEDULE-STATE] Medium\n> - `08:00` to `18:00` MON-FRI\n"
)

// TestConstraintsFollowTheRun pins what each input-affecting flag does to the output's constraints.
func TestConstraintsFollowTheRun(t *testing.T) {
	t.Parallel()

	builtIn := []string{"Off", "Minimum", "Medium", "Maximum"}

	for name, tc := range map[string]struct {
		want schedule.Constraints
		doc  string
		args []string
	}{
		"defaults": {
			doc:  constraintsDoc,
			want: schedule.Constraints{States: builtIn, SampleInterval: 1, DefaultBase: new("Off")},
		},
		"interval": {
			doc:  constraintsDoc,
			args: []string{"--sample-interval", "15"},
			want: schedule.Constraints{States: builtIn, SampleInterval: 15, DefaultBase: new("Off")},
		},
		"base in the configured spelling": {
			doc:  constraintsDoc,
			args: []string{"--base", "minimum"},
			want: schedule.Constraints{States: builtIn, SampleInterval: 1, DefaultBase: new("Minimum")},
		},
		"required base has no default": {
			doc:  withBaseDoc,
			args: []string{"--require-base", "true"},
			want: schedule.Constraints{States: builtIn, SampleInterval: 1, RequireBase: true},
		},
		"pattern as given": {
			doc:  constraintsDoc,
			args: []string{"--error-unless-alert-pattern", "(?i)NOTE|TIP"},
			want: schedule.Constraints{
				States: builtIn, SampleInterval: 1, DefaultBase: new("Off"), AlertPattern: new("(?i)NOTE|TIP"),
			},
		},
		"empty pattern differs from none": {
			doc:  constraintsDoc,
			args: []string{"--error-unless-alert-pattern", ""},
			want: schedule.Constraints{
				States: builtIn, SampleInterval: 1, DefaultBase: new("Off"), AlertPattern: new(""),
			},
		},
		"own states": {
			doc: constraintsDoc,
			args: []string{
				"--set-config-at", "states", "--set-config-value",
				`[{"id":"Idle","display-name":"Idle"},{"id":"Medium","display-name":"Medium"}]`,
			},
			want: schedule.Constraints{
				States: []string{"Idle", "Medium"}, SampleInterval: 1, DefaultBase: new("Idle"),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out, diags := parseAndRun(tc.args, []byte(tc.doc))
			if out == nil {
				t.Fatalf("no output: %v", diags)
			}

			got, _ := json.Marshal(out.Constraints)
			want, _ := json.Marshal(tc.want)

			if !bytes.Equal(got, want) {
				t.Errorf("constraints\n got %s\nwant %s", got, want)
			}
		})
	}
}

// TestEverySampleIntervalFitsTheSchema: the output checks itself against its schema, so every interval the CLI accepts
// has to be one the schema allows. The schema may allow more; the CLI's own rule stays the divisors of 60.
func TestEverySampleIntervalFitsTheSchema(t *testing.T) {
	t.Parallel()

	const doc = "# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `00:00` to `00:00` MON\n"

	accepted := 0

	for g := 1; g <= 1500; g++ {
		opts, diags := Parse([]string{"--sample-interval", strconv.Itoa(g)})
		if diags.HasErrors() {
			continue
		}

		accepted++

		out, runDiags := Run(&opts, []byte(doc))
		if out == nil {
			t.Errorf("--sample-interval %d: %v", g, runDiags)

			continue
		}

		if out.Constraints.SampleInterval != g {
			t.Errorf("--sample-interval %d: output says %d", g, out.Constraints.SampleInterval)
		}
	}

	if accepted != 12 {
		t.Errorf("the CLI accepts %d intervals, want the 12 divisors of 60", accepted)
	}
}
