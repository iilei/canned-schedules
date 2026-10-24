// Package cli parses command-line arguments and runs the pipeline on one document.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/schedule"
)

const minutesPerHour = 60

var errNotBool = errors.New(`want "true" or "false"`)

// Options is the parsed command line.
type Options struct {
	Overrides []config.Override
	Args      []string
	Schedule  schedule.Options
}

// Parse parses args (without the program name). Unknown or malformed flags become invalid-flag errors.
func Parse(args []string) (Options, diag.List) {
	var (
		opts   Options
		at     []string
		values []string
	)

	fs := flag.NewFlagSet("canned-schedules", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&opts.Schedule.Base, "base", "", "base tier when the document has no BASE block")
	fs.IntVar(&opts.Schedule.Granularity, "granularity", 1, "minutes; must divide 60, every time must be a multiple")
	fs.Func("require-base", "true|false (default false): the document must contain a BASE block", func(s string) error {
		v, err := parseBool(s)
		opts.Schedule.RequireBase = v

		return err
	})
	fs.Func(
		"set-config-at",
		"config path to override (repeatable, paired with --set-config-value)",
		func(s string) error {
			at = append(at, s)

			return nil
		},
	)
	fs.Func("set-config-value", "JSON value for the matching --set-config-at (repeatable)", func(s string) error {
		values = append(values, s)

		return nil
	})

	if err := fs.Parse(args); err != nil {
		return opts, diag.List{diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag, "%v", err)}
	}

	opts.Args = fs.Args()

	var diags diag.List

	if len(at) != len(values) {
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"%d --set-config-at but %d --set-config-value: they must come in pairs", len(at), len(values)))
	} else {
		for i := range at {
			opts.Overrides = append(opts.Overrides, config.Override{Path: at[i], Value: values[i]})
		}
	}

	if g := opts.Schedule.Granularity; g < 1 || minutesPerHour%g != 0 {
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"--granularity %d: must divide 60 (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60)", g))
	}

	if opts.Schedule.RequireBase && opts.Schedule.Base != "" {
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"--base can't be combined with --require-base true"))
	}

	return opts, diags
}

// Run evaluates one document. It returns the output (nil on errors), all diagnostics, and an error only for
// failures that aren't about the input, such as a stage that isn't implemented yet.
func Run(args []string, src []byte) (*schedule.Output, diag.List, error) {
	opts, diags := Parse(args)
	if diags.HasErrors() {
		return nil, diags, nil
	}

	cfg, cfgDiags := config.Load(opts.Overrides)
	diags = append(diags, cfgDiags...)

	if cfg == nil {
		return nil, diags, nil
	}

	if b := opts.Schedule.Base; b != "" {
		if _, ok := cfg.Tier(b); !ok {
			return nil, append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag, "--base %q: no such tier", b)), nil
		}
	}

	out, evalDiags, err := schedule.Evaluate(src, cfg, opts.Schedule)
	if err != nil {
		return nil, diags, fmt.Errorf("evaluate: %w", err)
	}

	diags = append(diags, evalDiags...)
	if diags.HasErrors() {
		return nil, diags, nil
	}

	return out, diags, nil
}

// parseBool accepts exactly "true" or "false"; strconv.ParseBool would also take "1", "t", "TRUE" and so on.
func parseBool(s string) (bool, error) {
	switch s {
	case strconv.FormatBool(true):
		return true, nil
	case strconv.FormatBool(false):
		return false, nil
	default:
		return false, errNotBool
	}
}
