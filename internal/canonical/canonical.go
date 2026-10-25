// Package canonical writes the canonical form of a valid request: only what the contract defines (the name, META,
// BASE and the STATE blocks with their ranges), in source order and with normalised spelling. Prose, other alerts and
// links are dropped. The result is itself a valid request that evaluates to the same output, digest included.
//
// The text form (Text) adds the ASCII preview in a fenced code block after the name. A top-level code block is prose
// to the parser, so the text form is a valid request as well.
package canonical

import (
	"errors"
	"fmt"
	"strings"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/document"
	"github.com/iilei/canned-schedules/internal/expr"
)

const minutesPerHour = 60

// ErrInvalid is returned for a request with errors; only valid requests have a canonical form.
var ErrInvalid = errors.New("the request has errors")

// Render parses src and writes its canonical form. State names are written as the config spells them, times as
// HH:MM, weekday expressions without whitespace (SAT, SUN becomes SAT,SUN); the grouping as written stays, so a
// repeated-weekday warning stays too. Ranges keep their order, because order decides the painting.
func Render(src []byte, cfg *config.Config) (string, error) {
	doc, diags := document.Parse(src, document.Options{})
	if diags.HasErrors() {
		return "", ErrInvalid
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n", clean(doc.Name.Value))

	if m := doc.Meta; m != nil {
		marker(&b, document.KindMeta, m.Title.Value)

		for _, item := range m.Items {
			key, value, ok := metaItem(item)
			if !ok {
				return "", ErrInvalid
			}

			fmt.Fprintf(&b, "> - %s: %s\n", key, codeSpan(value))
		}
	}

	if base := doc.Base; base != nil {
		marker(&b, document.KindBase, stateID(cfg, base.Title.Value))
	}

	for _, s := range doc.States {
		marker(&b, document.KindState, stateID(cfg, s.Title.Value))

		for _, item := range s.Items {
			line, err := rangeItem(item)
			if err != nil {
				return "", err
			}

			b.WriteString("> - " + line + "\n")
		}
	}

	return b.String(), nil
}

// Text inserts preview, fenced, after the H1 of the canonical form c. The fence is longer than any run of tildes
// that starts a line of the preview, so the preview can't close it.
func Text(c string, preview []byte) string {
	head, rest, _ := strings.Cut(c, "\n")

	body := string(preview)
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	fence := "~~~"
	for line := range strings.SplitSeq(body, "\n") {
		for strings.HasPrefix(strings.TrimLeft(line, " "), fence) {
			fence += "~"
		}
	}

	return head + "\n\n" + fence + "text\n" + body + fence + "\n" + rest
}

// marker starts a directive block, separated from the one before by a blank line.
func marker(b *strings.Builder, kind document.Kind, title string) {
	fmt.Fprintf(b, "\n> [!%s-%s]", document.Namespaces[0], kind)

	if title != "" {
		b.WriteString(" " + clean(title))
	}

	b.WriteString("\n")
}

// metaItem returns the key and value of a META item ("key: `value`").
func metaItem(item document.Item) (string, string, bool) {
	var key, value string

	var seenKey, seenValue bool

	for _, tok := range item.Tokens {
		switch {
		case tok.Kind == document.TokenText && !seenValue:
			key, seenKey = document.TrimSpace(strings.TrimSuffix(document.TrimSpace(tok.Value), ":")), true
		case tok.Kind == document.TokenCode && seenKey && !seenValue:
			value, seenValue = tok.Value, true
		}
	}

	return clean(key), clean(value), seenKey && seenValue && key != ""
}

// rangeItem writes a range item as "`HH:MM` to `HH:MM` DAYS".
func rangeItem(item document.Item) (string, error) {
	r, diags := expr.Parse(item, 1)
	if diags.HasErrors() {
		return "", ErrInvalid
	}

	// A valid item is code, " to ", code, and the weekday text.
	days := strings.Map(func(c rune) rune {
		if document.IsSpace(c) {
			return -1
		}

		return c
	}, item.Tokens[len(item.Tokens)-1].Value)

	return fmt.Sprintf("%#q to %#q %s", clock(r.Start), clock(r.End), days), nil
}

// stateID returns the state id as the config spells it; the request is valid, so the state exists.
func stateID(cfg *config.Config, name string) string {
	if s, ok := cfg.State(name); ok {
		return s.ID
	}

	return name
}

// codeSpan writes v as a code span: the backtick fence is longer than any run of backticks inside, and a value that
// starts or ends with a backtick or a space is padded with one space on each side, which the parser strips again.
func codeSpan(v string) string {
	longest, run := 0, 0
	for _, c := range v {
		if c == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}

	fence := strings.Repeat("`", longest+1)

	edge := func(c byte) bool { return c == '`' || c == ' ' }
	if v != "" && strings.Trim(v, " ") != "" && (edge(v[0]) || edge(v[len(v)-1])) {
		v = " " + v + " "
	}

	return fence + v + fence
}

func clock(mins int) string {
	return fmt.Sprintf("%02d:%02d", mins/minutesPerHour, mins%minutesPerHour)
}

// clean escapes control, line-separator and bidi characters (diag.Sanitize), so nothing written can start a new
// line or reorder the text.
func clean(s string) string {
	return diag.Sanitize(s)
}
