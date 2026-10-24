// Package report turns diagnostics into an error report (txt, JSON or the built-in Markdown template) and writes it
// safely. The CLI never posts reports anywhere; integrations take the file and do that themselves.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"text/template"

	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/templates"
)

// SchemaVersion is the version of the JSON report shape (schema/report.schema.json).
const SchemaVersion = "1"

// Formats and templates accepted by --error-output-format and --error-report-template.
const (
	FormatTXT           = "txt"
	FormatJSON          = "json"
	TemplateMarkdownGFM = "markdown-gfm"
)

// Report is the JSON error report. The Markdown template operates on exactly this shape. Field order is the JSON key
// order people read.
type Report struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	SchemaVersion string  `json:"schema-version"`
	File          string  `json:"file,omitempty"`
	Summary       Summary `json:"summary"`
	Diagnostics   []Entry `json:"diagnostics"`
}

// Summary counts the diagnostics by severity.
type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

// Entry is one diagnostic. Line, Column, Location and Excerpt are omitted for diagnostics without a position.
// File, Location, Message and Excerpt are sanitized (see diag.Sanitize).
type Entry struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	Severity string        `json:"severity"`
	Code     string        `json:"code"`
	File     string        `json:"file,omitempty"`
	Location string        `json:"location,omitempty"`
	Line     int           `json:"line,omitempty"`
	Column   int           `json:"column,omitempty"`
	Message  string        `json:"message"`
	Excerpt  *diag.Excerpt `json:"excerpt,omitempty"`
}

// Build creates the report for file. src may be nil when the file couldn't be read.
func Build(file string, src []byte, diags diag.List) Report {
	r := Report{SchemaVersion: SchemaVersion, File: diag.Sanitize(file), Diagnostics: []Entry{}}
	lines := diag.SourceLines(src)

	for _, d := range diags {
		switch d.Severity {
		case diag.Error:
			r.Summary.Errors++
		case diag.Warn:
			r.Summary.Warnings++
		}

		e := Entry{Severity: string(d.Severity), Code: d.Code, File: r.File, Message: diag.Sanitize(d.Message)}
		if d.Pos != (diag.Pos{}) {
			e.Line, e.Column = d.Pos.Line, d.Pos.Col
			e.Location = d.Location(file)
			e.Excerpt = diag.MakeExcerpt(lines, d.Pos)
		}

		r.Diagnostics = append(r.Diagnostics, e)
	}

	return r
}

// Render writes the report in the given format, or with the given built-in template when tmpl is not empty.
func Render(w io.Writer, r Report, format, tmpl string) error {
	switch {
	case tmpl == TemplateMarkdownGFM:
		return renderTemplate(w, r, templates.MarkdownGFM)
	case tmpl != "":
		return fmt.Errorf("unknown report template %q", tmpl)
	case format == FormatJSON:
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return fmt.Errorf("encode report: %w", err)
		}

		_, err = fmt.Fprintf(w, "%s\n", b)

		return err
	case format == FormatTXT || format == "":
		for i := range r.Diagnostics {
			if _, err := fmt.Fprintln(w, txtLine(&r.Diagnostics[i])); err != nil {
				return err
			}
		}

		return nil
	default:
		return fmt.Errorf("unknown report format %q", format)
	}
}

// txtLine renders an entry like diag.Format. Entry fields are already sanitized.
func txtLine(e *Entry) string {
	prefix := e.Location
	if prefix == "" {
		prefix = e.File
	}

	if prefix == "" {
		prefix = "canned-schedules"
	}

	return prefix + ": " + e.Severity + "[" + e.Code + "]: " + e.Message
}

// renderTemplate executes tmpl on the report decoded from its own JSON, so the template sees exactly the JSON shape
// (keys like .summary.errors), the same data gomplate gets with `-c .=report.json`. Missing keys are errors, as in
// gomplate, so the template has to read optional keys with index.
func renderTemplate(w io.Writer, r Report, tmpl string) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}

	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("decode report: %w", err)
	}

	t, err := template.New("report").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("parse report template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("render report template: %w", err)
	}

	_, err = w.Write(buf.Bytes())

	return err
}
