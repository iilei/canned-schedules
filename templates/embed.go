// Package templates embeds the repo-baked report templates.
package templates

import _ "embed" // Required for go:embed.

// MarkdownGFM renders the error report as GitHub-flavoured Markdown. It operates on the JSON report's data shape.
//
//go:embed markdown-gfm.tmpl
var MarkdownGFM string
