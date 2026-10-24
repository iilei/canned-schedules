// Package assets embeds files shared by the preflight page and the Sphinx kit: the bundled fonts and their licences.
package assets

import "embed"

// Fonts holds fonts/*.woff2 and their licence texts. The page serves them from fonts/ (mise run web:build copies
// them), and the Sphinx kit ships the ones used inside schedule admonitions.
//
//go:embed fonts
var Fonts embed.FS
