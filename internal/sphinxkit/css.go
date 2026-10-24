// Package sphinxkit gives Sphinx docs the look of the preflight page's Sphinx view: the stylesheet for schedule
// admonitions (shared with the page) and a downloadable kit with a local Sphinx extension.
package sphinxkit

import (
	"strconv"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/templates"
)

var (
	// Page is the preflight page: data-theme on <html> with light, dark and system.
	Page = Target{
		Scope: ".rendered.sphinx ", Dark: `:root[data-theme="dark"]`, Auto: `:root[data-theme="system"]`,
		Fonts: "fonts/",
	}
	// Furo is the Furo Sphinx theme: data-theme on <body> with light, dark and auto.
	Furo = Target{Dark: `body[data-theme="dark"]`, Auto: `body:not([data-theme="light"])`, Fonts: "fonts/"}
)

// Target selects where the stylesheet is used. Only the selectors differ; the rules are the same.
type Target struct {
	// Scope prefixes every rule, e.g. ".rendered.sphinx " for the page; empty for Sphinx.
	Scope string
	// Dark selects the dark theme.
	Dark string
	// Auto selects the theme that follows prefers-color-scheme (used inside a dark media query).
	Auto string
	// Fonts is the URL prefix of the font files.
	Fonts string
}

// cssModel is the data of templates/sphinx-schedule.css.tmpl.
type cssModel struct {
	Scope string    `json:"scope"`
	Dark  string    `json:"dark"`
	Auto  string    `json:"auto"`
	Fonts string    `json:"fonts"`
	Tiers []cssTier `json:"tiers"`
}

type cssTier struct {
	Class string `json:"class"`
	Light string `json:"light"`
	Dark  string `json:"dark"`
}

// CSS renders the schedule admonition stylesheet for target, coloured from the config's tier styles (round robin).
func CSS(cfg *config.Config, target Target) (string, error) {
	m := cssModel{Scope: target.Scope, Dark: target.Dark, Auto: target.Auto, Fonts: target.Fonts}

	for i := range cfg.Tiers {
		c := cfg.Style(i).Color
		m.Tiers = append(m.Tiers, cssTier{Class: "tier-" + strconv.Itoa(i), Light: c.Light, Dark: c.Dark})
	}

	out, err := templates.Execute(templates.SphinxScheduleCSS, m)

	return string(out), err
}
