// Package cli parses command-line arguments and runs the pipeline on one document.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"
	"regexp/syntax"
	"strconv"

	"github.com/iilei/canned-schedules/internal/canonical"
	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/myst"
	"github.com/iilei/canned-schedules/internal/render"
	"github.com/iilei/canned-schedules/internal/report"
	"github.com/iilei/canned-schedules/internal/schedule"
	"github.com/iilei/canned-schedules/internal/sphinxkit"
)

const minutesPerHour = 60

var errNotBool = errors.New(`want "true" or "false"`)

// Options is the parsed command line.
type Options struct {
	Sphinx    SphinxOptions
	Preview   PreviewOptions
	Report    ReportOptions
	Overrides []config.Override
	Args      []string
	Schedule  schedule.Options
	// Help is --help (also -h): print the help and exit.
	Help bool
	// Version is --version: print the version and exit.
	Version bool
}

// pending holds flag values that Parse turns into options after parsing.
type pending struct {
	symbols string
	at      []string
	values  []string
}

// SphinxOptions configure the Sphinx outputs.
type SphinxOptions struct {
	// MySTFile is --myst-output-file: the request's schedule as MyST.
	MySTFile string
	// KitFile is --sphinx-kit-output-file: the Sphinx kit zip.
	KitFile string
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

// newFlagSet registers every flag. The usage strings are the reference text: --help and the README's flag table are
// generated from them (FlagDocs), so they are written as sentences, with Markdown code spans.
func newFlagSet(opts *Options, p *pending) *flag.FlagSet {
	fs := flag.NewFlagSet("canned-schedules", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.IntVar(&opts.Schedule.SampleInterval, "sample-interval", 1,
		"Every time must be a multiple of it; must divide 60 (e.g. `15`, `60`)")
	fs.Func("require-base", "`true`: the document must contain a BASE block; can't be combined with `--base`",
		func(s string) error {
			v, err := parseBool(s)
			opts.Schedule.RequireBase = v

			return err
		})
	fs.StringVar(&opts.Schedule.Base, "base", "", "Base state when the document has no BASE block")
	fs.Func("error-unless-alert-pattern", "Every alert other than a schedule directive must match (whole type, Go "+
		"syntax, `(?i)` for any case)", func(s string) error {
		re, err := regexp.Compile(`^(?:` + s + `)$`)
		opts.Schedule.AlertPattern = re

		// The syntax error alone: the full error quotes the anchored expression in backticks.
		if se := (*syntax.Error)(nil); errors.As(err, &se) {
			return errors.New(string(se.Code))
		}

		return err
	})
	fs.Func("set-config-at", "Config path to replace, e.g. `representation.styles.states[0]` (repeatable)",
		func(s string) error {
			p.at = append(p.at, s)

			return nil
		})
	fs.Func("set-config-value", "JSON value for the matching `--set-config-at` (repeatable)", func(s string) error {
		p.values = append(p.values, s)

		return nil
	})
	fs.StringVar(&p.symbols, "ascii-symbols", "", "Symbols in the ASCII preview: `ascii`, `braille` or `styles` "+
		"(shorthand for `--set-config-at representation.ascii.symbols`)")
	registerPreviewFlags(fs, &opts.Preview)
	fs.StringVar(&opts.Sphinx.MySTFile, "myst-output-file", "",
		"Write the schedule as MyST for Sphinx (only when the document has no errors)")
	fs.StringVar(&opts.Sphinx.KitFile, "sphinx-kit-output-file", "",
		"Write the Sphinx kit zip (only when the document has no errors)")
	fs.StringVar(&opts.Report.File, "error-output-file", "",
		"Write the error report to this file (always, even when empty)")
	fs.StringVar(&opts.Report.Format, "error-output-format", "", "Error report format: `txt` or `json`")
	fs.StringVar(&opts.Report.Template, "error-report-template", "",
		"Render the error report with the built-in `markdown-gfm` template instead")
	fs.BoolVar(&opts.Help, "help", false, "Print this help and exit")
	fs.BoolVar(&opts.Version, "version", false, "Print the version and exit")

	return fs
}

// Parse parses args (without the program name). Unknown or malformed flags become invalid-flag errors. -h and --help
// set Help, --version sets Version.
func Parse(args []string) (Options, diag.List) {
	var (
		opts Options
		p    pending
	)

	fs := newFlagSet(&opts, &p)
	if err := fs.Parse(args); errors.Is(err, flag.ErrHelp) {
		opts.Help = true // -h, which isn't registered: the flag package reports it as a help request.

		return opts, nil
	} else if err != nil {
		return opts, diag.List{diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag, "%v", err)}
	}

	opts.Args = fs.Args()

	var diags diag.List

	if len(p.at) != len(p.values) {
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"%d --set-config-at but %d --set-config-value: they must come in pairs", len(p.at), len(p.values)))
	} else {
		for i := range p.at {
			opts.Overrides = append(opts.Overrides, config.Override{Path: p.at[i], Value: p.values[i]})
		}
	}

	diags = append(diags, asciiSymbols(&opts, p.symbols)...)

	if g := opts.Schedule.SampleInterval; g < 1 || minutesPerHour%g != 0 {
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"--sample-interval %d: must divide 60 (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60)", g))
	}

	diags = append(diags, reportFlags(opts.Report)...)
	diags = append(diags, checkPreviewFlags(&opts.Preview)...)

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
		if _, ok := cfg.State(b); !ok {
			return nil, append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag, "--base %q: no such state", b))
		}
	}

	out, evalDiags := schedule.Evaluate(src, cfg, opts.Schedule)
	diags = append(diags, evalDiags...)

	if out == nil {
		return nil, diags
	}

	if opts.Preview.Requested() {
		if p := render.Downsample(out, cfg, opts.Preview.Downsample); p.Mixed > 0 {
			diags = append(diags, diag.Warnf(diag.Pos{}, diag.CodePreviewApproximate,
				"the preview uses %d-minute buckets: %d of them mix states and show the one covering most of the "+
					"bucket (the JSON stays exact; see --preview-downsample)", p.Size, p.Mixed))
		}
	}

	// The output also carries the config and preview warnings, which Evaluate doesn't see.
	out.Warnings = schedule.Warnings(src, diags)
	if err := out.Finish(); err != nil {
		return nil, append(diags, diag.Errorf(diag.Pos{}, diag.CodeOutputSchema,
			"internal error, please report it: the output fails its own schema: %v", err))
	}

	return out, diags
}

// SVG renders the SVG preview of a successfully evaluated document with the same config overrides. With
// --svg-embed-source it carries the canonical form of src and the generator (the tool and version).
func SVG(opts *Options, out *schedule.Output, src []byte, generator string) ([]byte, error) {
	cfg, err := previewConfig(opts)
	if err != nil {
		return nil, err
	}

	var source *render.Source

	if opts.Preview.SVGEmbedSource {
		c, err := canonical.Render(src, cfg)
		if err != nil {
			return nil, err
		}

		source = &render.Source{Text: c, Generator: generator}
	}

	return render.SVG(out, cfg, opts.Preview.Downsample, opts.Preview.SVGTheme, source)
}

// Canonical writes the canonical form of a successfully evaluated document.
func Canonical(opts *Options, src []byte) ([]byte, error) {
	cfg, err := previewConfig(opts)
	if err != nil {
		return nil, err
	}

	c, err := canonical.Render(src, cfg)

	return []byte(c), err
}

// Text writes the text form: the canonical request with the ASCII preview (ANSI colours when color is true).
func Text(opts *Options, out *schedule.Output, src []byte, color bool) ([]byte, error) {
	c, err := Canonical(opts, src)
	if err != nil {
		return nil, err
	}

	preview, err := ASCII(opts, out, color)
	if err != nil {
		return nil, err
	}

	return []byte(canonical.Text(string(c), preview)), nil
}

// ASCII renders the ASCII preview, with ANSI colours when color is true.
func ASCII(opts *Options, out *schedule.Output, color bool) ([]byte, error) {
	cfg, err := previewConfig(opts)
	if err != nil {
		return nil, err
	}

	return render.ASCII(out, cfg, opts.Preview.Downsample, color)
}

// MyST renders the schedule of a successfully evaluated document as MyST.
func MyST(opts *Options, src []byte) ([]byte, error) {
	cfg, err := previewConfig(opts)
	if err != nil {
		return nil, err
	}

	out, err := myst.Render(src, cfg)

	return []byte(out), err
}

// SphinxKit builds the Sphinx kit for a successfully evaluated document; version names the generating tool.
func SphinxKit(opts *Options, src []byte, version string) ([]byte, error) {
	cfg, err := previewConfig(opts)
	if err != nil {
		return nil, err
	}

	return sphinxkit.Kit(src, cfg, version)
}

// previewConfig reloads the config; Run already validated it, so this can't fail for a document that evaluated.
func previewConfig(opts *Options) (*config.Config, error) {
	cfg, diags := config.Load(opts.Overrides)
	if cfg == nil {
		return nil, fmt.Errorf("config: %v", diags)
	}

	return cfg, nil
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
				"--error-output-format json and your own tooling, such as gomplate", r.Template))
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
