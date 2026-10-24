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
	"github.com/iilei/canned-schedules/internal/render"
	"github.com/iilei/canned-schedules/internal/report"
	"github.com/iilei/canned-schedules/internal/schedule"
)

const minutesPerHour = 60

var errNotBool = errors.New(`want "true" or "false"`)

// Options is the parsed command line.
type Options struct {
	// SVGFile is --svg-output-file; empty means no preview.
	SVGFile   string
	Report    ReportOptions
	Overrides []config.Override
	Args      []string
	Schedule  schedule.Options
}

// ReportOptions configure the error report.
type ReportOptions struct {
	// File is --error-output-file; empty means stderr.
	File string
	// Format is --error-output-format: txt (default) or json.
	Format string
	// Template is --error-report-template: empty or markdown-gfm. Mutually exclusive with Format.
	Template string
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
	fs.IntVar(
		&opts.Schedule.SampleInterval,
		"sample-interval",
		1,
		"minutes; must divide 60, every time must be a multiple",
	)
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
	fs.StringVar(&opts.SVGFile, "svg-output-file", "", "write an SVG preview of the schedule to this file (only "+
		"when the document has no errors)")
	fs.StringVar(&opts.Report.File, "error-output-file", "", "write the error report to this file instead of stderr")
	fs.StringVar(&opts.Report.Format, "error-output-format", "", "error report format: txt (default) or json")
	fs.StringVar(&opts.Report.Template, "error-report-template", "", "render the error report with a built-in "+
		"template: markdown-gfm")
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

	if g := opts.Schedule.SampleInterval; g < 1 || minutesPerHour%g != 0 {
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"--sample-interval %d: must divide 60 (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60)", g))
	}

	diags = append(diags, reportFlags(opts.Report)...)

	if opts.Schedule.RequireBase && opts.Schedule.Base != "" {
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"--base can't be combined with --require-base true"))
	}

	return opts, diags
}

// Run evaluates one document with parsed options. It returns the output (nil on errors) and all diagnostics.
func Run(opts *Options, src []byte) (*schedule.Output, diag.List) {
	cfg, diags := config.Load(opts.Overrides)
	if cfg == nil {
		return nil, diags
	}

	if b := opts.Schedule.Base; b != "" {
		if _, ok := cfg.Tier(b); !ok {
			return nil, append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag, "--base %q: no such tier", b))
		}
	}

	out, evalDiags := schedule.Evaluate(src, cfg, opts.Schedule)
	diags = append(diags, evalDiags...)

	if out == nil {
		return nil, diags
	}

	// The output also carries the config warnings, which Evaluate doesn't see.
	out.Warnings = schedule.Warnings(src, diags)
	if err := out.Finish(src); err != nil {
		return nil, append(diags, diag.Errorf(diag.Pos{}, diag.CodeOutputSchema,
			"internal error, please report it: the output fails its own schema: %v", err))
	}

	return out, diags
}

// SVG renders the preview of a successfully evaluated document with the same config overrides.
func SVG(opts *Options, out *schedule.Output) ([]byte, error) {
	cfg, diags := config.Load(opts.Overrides)
	if cfg == nil {
		return nil, fmt.Errorf("config: %v", diags)
	}

	return render.SVG(out, cfg)
}

func reportFlags(r ReportOptions) diag.List {
	var diags diag.List

	switch r.Format {
	case "", report.FormatTXT, report.FormatJSON:
	default:
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"--error-output-format %q: use txt or json", r.Format))
	}

	switch r.Template {
	case "", report.TemplateMarkdownGFM:
	default:
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"--error-report-template %q: the only built-in template is markdown-gfm; for other layouts use "+
				"--error-output-format json and your own tooling (e.g. gomplate)", r.Template))
	}

	if r.Format != "" && r.Template != "" {
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"--error-output-format and --error-report-template can't be combined"))
	}

	return diags
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
