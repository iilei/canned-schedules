// Package myst renders the schedule of a valid request as MyST Markdown for Sphinx: the name, the hint and the
// META, BASE and TIER blocks as admonitions with the classes the Sphinx kit styles. The request's free text (editing
// hints, syntax reference) is left out; it belongs to the issue, not to a handbook page.
package myst

import (
	"errors"
	"fmt"
	"strings"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/document"
)

// BaseText is the body of the BASE admonition (docutils requires content in an admonition), shared with the page's
// Sphinx view (internal/htmlpreview).
const BaseText = "Applies wherever no range is set."

// ErrInvalid is returned for a request with errors; only valid requests are rendered.
var ErrInvalid = errors.New("the request has errors")

// Render parses src and renders its schedule. It needs MyST's colon_fence extension. Items are written
// as in the request: weekdays get no {abbr} roles, which would only add noise to a file people read.
func Render(src []byte, cfg *config.Config) (string, error) {
	doc, diags := document.Parse(src)
	if diags.HasErrors() {
		return "", ErrInvalid
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n", doc.Name.Value)

	if m := doc.Meta; m != nil {
		switch {
		case len(m.Items) > 0:
			title := "Schedule"
			if m.Title.Value != "" {
				title += ": " + m.Title.Value
			}

			admonition(&b, title, "schedule schedule-meta", m.Items, "")
		case m.Title.Value != "":
			// docutils requires content in an admonition: a hint alone is a paragraph.
			fmt.Fprintf(&b, "\n%s\n", m.Title.Value)
		}
	}

	if base := doc.Base; base != nil {
		class, name := tier(cfg, base.Title.Value)
		admonition(&b, "Base tier: "+name, "schedule schedule-base "+class, nil, BaseText)
	}

	for _, t := range doc.Tiers {
		class, name := tier(cfg, t.Title.Value)
		admonition(&b, name, "schedule schedule-tier "+class, t.Items, "")
	}

	return b.String(), nil
}

// admonition writes one colon-fence admonition. Titles and text are sanitized, which turns newlines into a visible
// \u000a escape: display names come from the config and may contain anything, and a newline would end the
// directive argument or inject options.
func admonition(b *strings.Builder, title, class string, items []document.Item, body string) {
	fmt.Fprintf(b, "\n:::{admonition} %s\n:class: %s\n", strings.TrimSpace(diag.Sanitize(title)), class)

	if body != "" {
		b.WriteString(body + "\n")
	}

	for _, item := range items {
		b.WriteString("- ")

		for _, tok := range item.Tokens {
			switch tok.Kind {
			case document.TokenCode:
				b.WriteString("`" + tok.Value + "`")
			case document.TokenText:
				b.WriteString(diag.Sanitize(tok.Value))
			case document.TokenOther:
			}
		}

		b.WriteString("\n")
	}

	b.WriteString(":::\n")
}

// tier returns the CSS class and display name of a tier id; the request is valid, so the tier exists.
func tier(cfg *config.Config, id string) (string, string) {
	for i, t := range cfg.Tiers {
		if strings.EqualFold(t.ID, id) {
			return fmt.Sprintf("tier-%d", i), t.DisplayName
		}
	}

	return "tier-unknown", id
}
