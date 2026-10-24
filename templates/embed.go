// Package templates embeds the repo-baked report templates.
package templates

import _ "embed" // Required for go:embed.

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
