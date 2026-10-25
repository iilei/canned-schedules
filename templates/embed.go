// Package templates embeds the repo-baked templates: the Markdown error report, the SVG and ASCII previews, and the
// Sphinx schedule stylesheet. They are plain text/template, gomplate-compatible, and run on JSON-shaped models
// (Execute).
package templates

import (
	"bytes"
	_ "embed" // Required for go:embed.
	"encoding/json"
	"fmt"
	"text/template"
)

// MarkdownGFM renders the error report as GitHub-flavoured Markdown. It operates on the JSON report's data shape.
//
//go:embed markdown-gfm.tmpl
var MarkdownGFM string

// ScheduleSVG renders a schedule preview as SVG. It operates on the render model computed by internal/render.
//
//go:embed schedule-svg.tmpl
var ScheduleSVG string

// ScheduleASCII renders a schedule preview as text. It operates on the ASCII model computed by internal/render.
//
//go:embed schedule-ascii.tmpl
var ScheduleASCII string

// SphinxScheduleCSS styles schedule admonitions in Sphinx (Furo) and in the page's Sphinx view.
//
//go:embed sphinx-schedule.css.tmpl
var SphinxScheduleCSS string

// Execute runs tmpl on model decoded from its own JSON, so the template sees exactly the JSON shape: the same data
// gomplate gets with `-c .=model.json`. Missing keys are errors, as in gomplate.
func Execute(tmpl string, model any) ([]byte, error) {
	raw, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("encode template data: %w", err)
	}

	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode template data: %w", err)
	}

	t, err := template.New("template").Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render template: %w", err)
	}

	return buf.Bytes(), nil
}
