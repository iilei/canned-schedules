package cannedschedules

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/report"
	"github.com/iilei/canned-schedules/internal/schedule"
)

// Severities.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warn"
)

// Output is the canonical output of a schedule (schema/output.schema.json). Its JSON encoding has the output's
// keys; Result.JSON gives the exact text the CLI prints.
type Output struct {
	SchemaVersion string `json:"schema-version"`
	// Name is the schedule's H1.
	Name string `json:"name"`
	// Summary is the text after the META marker; empty without one.
	Summary string `json:"summary,omitempty"`
	// Annotations holds the META block's x-* keys as written, uninterpreted.
	Annotations map[string]string `json:"annotations"`
	// Base is the base state's id.
	Base string `json:"base"`
	// Week is the painted week: per day, its runs of one state.
	Week Week `json:"week"`
	// Transitions are the times the week switches state, with cron expressions.
	Transitions []Transition `json:"transitions"`
	// Warnings lists the warnings, sorted by position; they name no file.
	Warnings []Warning `json:"warnings"`
}

// Week lists the runs of each day, Monday first.
type Week struct {
	MON []Run `json:"MON"`
	TUE []Run `json:"TUE"`
	WED []Run `json:"WED"`
	THU []Run `json:"THU"`
	FRI []Run `json:"FRI"`
	SAT []Run `json:"SAT"`
	SUN []Run `json:"SUN"`
}

// Run is a half-open [Start, End) stretch of one day in one state, times as HH:MM. End "00:00" means the end of the
// day.
type Run struct {
	Start string `json:"start"`
	End   string `json:"end"`
	State string `json:"state"`
}

// Transition is a switch to State at Time (HH:MM) on Days (full list, Monday first).
type Transition struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	Time  string   `json:"time"`
	Days  []string `json:"days"`
	State string   `json:"state"`
	Cron  Cron     `json:"cron"`
}

// Cron holds a transition in two cron dialects, both without a time zone.
type Cron struct {
	// POSIX is five-field cron with numeric weekdays (0 = SUN), as EC2 Auto Scaling takes it.
	POSIX string `json:"posix"`
	// AWS is the six-field EventBridge format with weekday names.
	AWS string `json:"aws"`
}

// Warning is a warning in the output. Line and Column are 0 for warnings without a position.
type Warning struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	Code    string   `json:"code"`
	Line    int      `json:"line,omitempty"`
	Column  int      `json:"column,omitempty"`
	Message string   `json:"message"`
	Example string   `json:"example,omitempty"`
	Excerpt *Excerpt `json:"excerpt,omitempty"`
}

// Excerpt is the source line of a positioned diagnostic and a marker line with a caret under the column.
type Excerpt struct {
	Text   string `json:"text"`
	Marker string `json:"marker"`
}

// Severity is "error" or "warn", the spelling every output uses.
type Severity string

// Diagnostic is one error or warning. Message and Example are sanitized: control, line separator and bidi
// characters are escaped as \uXXXX, so echoing them can't forge log lines or workflow commands.
type Diagnostic struct {
	Severity Severity
	// Code is stable (the README's diagnostic table); messages are not.
	Code string
	// Message is plain prose.
	Message string
	// Example is the exact text to type instead (Markdown); empty when there is none.
	Example string
	// Line and Column are 1-based (columns count runes); both 0 when the diagnostic has no position.
	Line   int
	Column int
}

// Report is the error report of an evaluation (schema/report.schema.json), in the CLI's three formats.
type Report struct {
	r report.Report
}

// String renders the diagnostic as the CLI's txt report does, without a file: "line:col: severity[code]: message".
func (d Diagnostic) String() string {
	return diag.Diagnostic{
		Severity: map[Severity]diag.Severity{SeverityError: diag.Error, SeverityWarning: diag.Warn}[d.Severity],
		Code:     d.Code, Message: d.Message, Example: d.Example, Pos: diag.Pos{Line: d.Line, Col: d.Column},
	}.Format("")
}

// Errors counts the errors in the report.
func (p *Report) Errors() int { return p.r.Summary.Errors }

// Warnings counts the warnings in the report.
func (p *Report) Warnings() int { return p.r.Summary.Warnings }

// JSON is the report as --error-output-format json writes it.
func (p *Report) JSON() ([]byte, error) { return p.render(report.FormatJSON, "") }

// Markdown is the report rendered with the built-in markdown-gfm template (--error-report-template); every
// user-derived string sits inside a fence.
func (p *Report) Markdown() ([]byte, error) { return p.render("", report.TemplateMarkdownGFM) }

// Text is the txt report: one "path:line:col: severity[code]: message" line per diagnostic.
func (p *Report) Text() ([]byte, error) { return p.render(report.FormatTXT, "") }

func (p *Report) render(format, tmpl string) ([]byte, error) {
	var buf bytes.Buffer
	if err := report.Render(&buf, p.r, format, tmpl); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func diagnostics(l diag.List) []Diagnostic {
	ds := make([]Diagnostic, 0, len(l))
	for _, d := range l {
		ds = append(ds, Diagnostic{
			Severity: Severity(d.Severity.String()), Code: d.Code,
			Message: diag.Sanitize(d.Message), Example: diag.Sanitize(d.Example),
			Line: d.Pos.Line, Column: d.Pos.Col,
		})
	}

	return ds
}

// fromInternal and toInternal convert through JSON: both types have the same keys, and TestOutputMirrorsInternal
// checks that nothing is lost.
func fromInternal(out *schedule.Output) (*Output, error) {
	var o Output

	return &o, recode(out, &o)
}

func toInternal(o *Output) (*schedule.Output, error) {
	var out schedule.Output

	return &out, recode(o, &out)
}

func recode(from, to any) error {
	raw, err := json.Marshal(from)
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}

	if err := json.Unmarshal(raw, to); err != nil {
		return fmt.Errorf("decode output: %w", err)
	}

	return nil
}
