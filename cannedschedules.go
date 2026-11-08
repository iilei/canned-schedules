// Package cannedschedules evaluates weekly schedules written in Markdown and renders them: the canonical output
// (the same JSON the canned-schedules CLI prints), diagnostics with positions, the error report, the canonical
// Markdown, the text form and the ASCII and SVG previews.
//
// It is the library behind the CLI's pipeline, for tools that build on canned schedules without running the binary,
// such as companions that post previews to issues. It never touches the network or the file system: the caller
// reads the document and writes the results.
//
// The exported API is the contract, under the module's semver: before 1.0 a breaking change raises the minor
// version. The data shapes have their own contract, schema-version (see SchemaVersion and the README's
// "Stability"); Output mirrors the output schema field by field.
package cannedschedules

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"

	"github.com/iilei/canned-schedules/internal/cli"
	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/report"
	"github.com/iilei/canned-schedules/internal/schedule"
	"github.com/iilei/canned-schedules/schema"
)

// SchemaVersion is the date of the last breaking change to the data contracts (document META, config, output,
// report). Output.SchemaVersion and the report carry it.
const SchemaVersion = schema.Version

// DefaultDownsample is the preview bucket size in minutes when PreviewOptions.Downsample is 0, as in the CLI.
const DefaultDownsample = cli.DefaultDownsample

const minutesPerHour = 60

var (
	// ErrInvalidOptions wraps every error about unusable Options or SVGOptions.
	ErrInvalidOptions = errors.New("invalid options")
	// ErrInvalidDocument is returned by the renderers of a Result whose document has errors.
	ErrInvalidDocument = errors.New("the document has errors")
	// ErrNoPreview is returned by the preview renderers of a Result evaluated without Options.Preview.
	ErrNoPreview = errors.New("no preview planned: set Options.Preview")
)

// Options configure an evaluation. Nil or the zero value match the CLI's defaults without any preview.
type Options struct {
	// Preview, when set, plans previews (SVG, ASCII, text form) and configures all of them. Only then can the
	// Result render previews, and only then does the output carry the preview-approximate warning when buckets mix
	// states, as with the CLI's preview flags.
	Preview *PreviewOptions
	// Base names the base state for a document without a BASE block (the CLI's --base). Empty means the first
	// state in the config. Can't be combined with RequireBase.
	Base string
	// AlertPattern, when not empty, makes alerts strict: every alert other than a schedule directive must match it
	// (--error-unless-alert-pattern). Go RE2 syntax; the library anchors it.
	AlertPattern string
	// Config replaces values of the built-in config, in order, like pairs of --set-config-at and
	// --set-config-value: a dot-separated path with [index] and a JSON value. Problems with them are diagnostics.
	Config []ConfigOverride
	// SampleInterval is the grid in minutes every time must sit on (--sample-interval); it must divide 60. 0 means 1.
	SampleInterval int
	// RequireBase makes the BASE block mandatory (--require-base true).
	RequireBase bool
}

// ConfigOverride replaces the config value at Path with the JSON document Value.
type ConfigOverride struct {
	// Path is dot-separated keys plus [index], e.g. "representation.styles.states[0]".
	Path string
	// Value is JSON, e.g. `[{"id": "Off", "display-name": "Off"}]`.
	Value string
}

// PreviewOptions configure every preview of a Result.
type PreviewOptions struct {
	// Downsample is the bucket size in minutes (--preview-downsample=N); it must divide 60. 0 means
	// DefaultDownsample, 1 means exact (--no-preview-downsample).
	Downsample int
}

// Result is an evaluated document. It is safe for concurrent use.
type Result struct {
	// Output is the canonical output; nil when the document has errors.
	Output *Output
	out    *schedule.Output
	// Diagnostics lists every error and warning, in the CLI's order.
	Diagnostics []Diagnostic
	src         []byte
	diags       diag.List
	opts        cli.Options
	preview     bool
}

// Evaluate checks and evaluates the schedule document src; opts may be nil. Problems in the document or in the
// config overrides are diagnostics in the Result; the error is only for unusable Options and wraps
// ErrInvalidOptions.
func Evaluate(src []byte, opts *Options) (*Result, error) {
	if opts == nil {
		opts = &Options{}
	}

	c, err := cliOptions(opts)
	if err != nil {
		return nil, err
	}

	preview := 0
	if opts.Preview != nil {
		preview = c.Preview.Downsample
	}

	out, diags := cli.Evaluate(c.Overrides, c.Schedule, preview, src)
	r := &Result{opts: c, src: src, diags: diags, preview: opts.Preview != nil, Diagnostics: diagnostics(diags)}

	if out != nil {
		r.out = out

		if r.Output, err = fromInternal(out); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// Valid reports whether the document has no errors (warnings allowed).
func (r *Result) Valid() bool { return r.out != nil }

// JSON is the canonical output exactly as the CLI prints it.
func (r *Result) JSON() ([]byte, error) {
	if r.out == nil {
		return nil, ErrInvalidDocument
	}

	return cli.JSON(r.out)
}

// Canonical is the schedule in canonical form: the Markdown the contract defines (name, META, BASE, STATE blocks
// and ranges in source order) without prose, other alerts or the preview.
func (r *Result) Canonical() ([]byte, error) {
	if r.out == nil {
		return nil, ErrInvalidDocument
	}

	return cli.Canonical(&r.opts, r.src)
}

// Text is the text form: the canonical form with the ASCII preview after the name, itself a valid document. color
// adds ANSI colours to the preview.
func (r *Result) Text(color bool) ([]byte, error) {
	if err := r.canPreview(); err != nil {
		return nil, err
	}

	return cli.Text(&r.opts, r.out, r.src, color)
}

// ASCII is the ASCII preview of the week. color adds ANSI colours.
func (r *Result) ASCII(color bool) ([]byte, error) {
	if err := r.canPreview(); err != nil {
		return nil, err
	}

	return cli.ASCII(&r.opts, r.out, color)
}

// Report is the error report of the evaluation; file names the document in it and may be empty.
func (r *Result) Report(file string) *Report {
	return &Report{r: report.Build(file, r.src, r.diags)}
}

func (r *Result) canPreview() error {
	switch {
	case r.out == nil:
		return ErrInvalidDocument
	case !r.preview:
		return ErrNoPreview
	}

	return nil
}

// cliOptions checks opts and converts them; the checks are those the CLI applies to its flags.
func cliOptions(opts *Options) (cli.Options, error) {
	var c cli.Options

	c.Schedule.SampleInterval = opts.SampleInterval
	if c.Schedule.SampleInterval == 0 {
		c.Schedule.SampleInterval = 1
	}

	if err := divides60("SampleInterval", c.Schedule.SampleInterval); err != nil {
		return c, err
	}

	if opts.RequireBase && opts.Base != "" {
		return c, fmt.Errorf("%w: Base can't be combined with RequireBase", ErrInvalidOptions)
	}

	c.Schedule.RequireBase, c.Schedule.Base = opts.RequireBase, opts.Base

	if opts.AlertPattern != "" {
		re, err := regexp.Compile(`^(?:` + opts.AlertPattern + `)$`)
		if se := (*syntax.Error)(nil); errors.As(err, &se) {
			return c, fmt.Errorf("%w: AlertPattern %q: %s", ErrInvalidOptions, opts.AlertPattern, se.Code)
		} else if err != nil {
			return c, fmt.Errorf("%w: AlertPattern %q: %w", ErrInvalidOptions, opts.AlertPattern, err)
		}

		c.Schedule.AlertPattern = re
	}

	for _, o := range opts.Config {
		c.Overrides = append(c.Overrides, config.Override{Path: o.Path, Value: o.Value})
	}

	c.Preview.Downsample = DefaultDownsample
	if p := opts.Preview; p != nil && p.Downsample != 0 {
		if err := divides60("Preview.Downsample", p.Downsample); err != nil {
			return c, err
		}

		c.Preview.Downsample = p.Downsample
	}

	return c, nil
}

func divides60(name string, n int) error {
	if n < 1 || minutesPerHour%n != 0 {
		return fmt.Errorf("%w: %s %d must divide 60 (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60)",
			ErrInvalidOptions, name, n)
	}

	return nil
}
