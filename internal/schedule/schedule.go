// Package schedule turns a schedule document into the canonical weekly output.
//
// Evaluate runs the stages in order: parse (internal/document), META validation, tier and base resolution, range
// expressions (internal/expr) and painting.
package schedule

import (
	"strings"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/document"
	"github.com/iilei/canned-schedules/internal/expr"
)

// SchemaVersion is the schema version built into the binary. It applies when a document doesn't declare one.
const SchemaVersion = "1"

// Options are the command-line settings that affect evaluation.
type Options struct {
	// Base is the --base tier; empty when not given.
	Base string
	// SampleInterval is --sample-interval in minutes; every time must be a multiple of it.
	SampleInterval int
	// RequireBase is --require-base.
	RequireBase bool
}

// Output is the canonical output of a schedule document. Field order is the JSON key order people read.
type Output struct {
	SchemaVersion string            `json:"schema-version"`
	Name          string            `json:"name"`
	Hint          string            `json:"hint,omitempty"`
	Base          string            `json:"base"`
	Annotations   map[string]string `json:"annotations"`
	Digests       Digests           `json:"digests"`
	Week          Week              `json:"week"`
	Transitions   []Transition      `json:"transitions"`
	Warnings      []Warning         `json:"warnings"`
}

// Warning is a non-fatal diagnostic carried in the canonical output. It names no file, so the output depends only on
// the document and the flags, not on where the file lives.
type Warning struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	Code    string        `json:"code"`
	Line    int           `json:"line,omitempty"`
	Column  int           `json:"column,omitempty"`
	Message string        `json:"message"`
	Example string        `json:"example,omitempty"`
	Excerpt *diag.Excerpt `json:"excerpt,omitempty"`
}

// Digests identify the request and the effective schedule.
type Digests struct {
	// Request is the SHA-256 of the exact input bytes: any edit, prose included, changes it.
	Request string `json:"request"`
	// Effective is the SHA-256 of the base tier and the painted week: it changes only when what a consumer acts on
	// changes, never for name, hint, annotations or cron rendering.
	Effective string `json:"effective"`
}

// Week lists the runs of each day, Monday first. Field order defines the JSON key order.
type Week struct {
	MON []Run `json:"MON"`
	TUE []Run `json:"TUE"`
	WED []Run `json:"WED"`
	THU []Run `json:"THU"`
	FRI []Run `json:"FRI"`
	SAT []Run `json:"SAT"`
	SUN []Run `json:"SUN"`
}

// Run is a half-open [Start, End) stretch of one day with a single tier. End "00:00" means the end of the day.
type Run struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Tier  string `json:"tier"`
}

// tierRange is one parsed range item together with the tier it paints.
type tierRange struct {
	tier config.Tier
	expr.Range
}

// Warnings converts the warnings among diags, sorted by position (those without a position first).
func Warnings(src []byte, diags diag.List) []Warning {
	sorted := append(diag.List(nil), diags...)
	sorted.Sort()

	lines := diag.SourceLines(src)
	out := []Warning{}

	for _, d := range sorted {
		if d.Severity != diag.Warn {
			continue
		}

		w := Warning{Code: d.Code, Message: diag.Sanitize(d.Message), Example: diag.Sanitize(d.Example)}
		if d.Pos != (diag.Pos{}) {
			w.Line, w.Column = d.Pos.Line, d.Pos.Col
			w.Excerpt = diag.MakeExcerpt(lines, d.Pos)
		}

		out = append(out, w)
	}

	return out
}

// Evaluate parses src and paints the week. The output is nil when there are errors.
func Evaluate(src []byte, cfg *config.Config, opts Options) (*Output, diag.List) {
	doc, diags := document.Parse(src)
	if diags.HasErrors() {
		return nil, diags
	}

	m, metaDiags := decodeMeta(doc.Meta)
	diags = append(diags, metaDiags...)

	ranges, tierDiags := resolveTiers(doc, cfg, opts.SampleInterval)
	diags = append(diags, tierDiags...)

	base, baseDiags := resolveBase(doc, cfg, opts)
	diags = append(diags, baseDiags...)

	diags.Sort()

	if diags.HasErrors() {
		return nil, diags
	}

	week := paint(base, ranges)
	out := &Output{
		SchemaVersion: m.schemaVersion,
		Name:          doc.Name.Value,
		Hint:          m.hint,
		Base:          base.ID,
		Annotations:   m.annotations,
		Week:          weekRuns(week),
		Transitions:   transitions(week),
		Warnings:      Warnings(src, diags),
	}

	if err := out.Finish(src); err != nil {
		return nil, append(diags, diag.Errorf(diag.Pos{}, diag.CodeOutputSchema,
			"internal error, please report it: the output fails its own schema: %v", err))
	}

	return out, diags
}

// resolveTiers maps every TIER block to its config tier and parses its range items, in source order.
func resolveTiers(doc *document.Document, cfg *config.Config, sampleInterval int) ([]tierRange, diag.List) {
	var (
		ranges []tierRange
		diags  diag.List
	)

	for _, d := range doc.Tiers {
		t, ok := cfg.Tier(d.Title.Value)
		if !ok {
			diags = append(diags, unknownTier(cfg, d.Title))
		}

		for _, item := range d.Items {
			r, rangeDiags := expr.Parse(item, sampleInterval)
			diags = append(diags, rangeDiags...)

			if ok && !rangeDiags.HasErrors() {
				ranges = append(ranges, tierRange{tier: t, Range: r})
			}
		}
	}

	return ranges, diags
}

// resolveBase picks the base tier: the document's BASE, then --base, then the first configured tier.
func resolveBase(doc *document.Document, cfg *config.Config, opts Options) (config.Tier, diag.List) {
	switch {
	case doc.Base != nil:
		t, ok := cfg.Tier(doc.Base.Title.Value)
		if !ok {
			return config.Tier{}, diag.List{unknownTier(cfg, doc.Base.Title)}
		}

		return t, nil
	case opts.RequireBase:
		return config.Tier{}, diag.List{diag.Errorf(diag.Pos{}, diag.CodeMissingBase,
			"--require-base true: the document needs a [!SCHEDULE:BASE] block")}
	case opts.Base != "":
		t, _ := cfg.Tier(opts.Base) // Validated by the CLI.

		return t, nil
	default:
		return cfg.Tiers[0], nil
	}
}

func unknownTier(cfg *config.Config, title document.Text) diag.Diagnostic {
	ids := make([]string, 0, len(cfg.Tiers))
	for _, t := range cfg.Tiers {
		ids = append(ids, t.ID)
	}

	return diag.Errorf(title.Pos, diag.CodeUnknownTier, "unknown tier %q; configured tiers: %s",
		title.Value, strings.Join(ids, ", "))
}
