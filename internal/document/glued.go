package document

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// gluedLine matches a quoted line that starts with a schedule marker: "> [!SCHEDULE-…]", possibly nested ("> > …").
var gluedLine = regexp.MustCompile(`(?i)^[ \t]{0,3}(?:>[ \t]?)+\[!(?:` + quoteAll(Namespaces) + `)[-\]]`)

// quotedLine matches a line that continues a quote explicitly.
var quotedLine = regexp.MustCompile(`^[ \t]{0,3}>`)

// Glued blocks: Markdown keeps a quote going through a line without ">" that follows it directly (a lazy
// continuation line, such as a stray "."), and through quoted lines without an empty line in between. A marker on
// such a line looks like the start of a new block but sits inside the block above. These helpers find those
// markers, so the block above stops before them and the error says what happened.

// span returns the first and last source line (1-based) of a block node.
func (p *parser) span(n ast.Node) (int, int) {
	from, to := 0, 0

	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || c.Type() != ast.TypeBlock {
			return ast.WalkContinue, nil
		}

		lines := c.Lines()
		for i := range lines.Len() {
			seg := lines.At(i)
			first, last := p.lines.pos(seg.Start).Line, p.lines.pos(max(seg.Start, seg.Stop-1)).Line

			if from == 0 || first < from {
				from = first
			}

			to = max(to, last)
		}

		return ast.WalkContinue, nil
	})

	return from, to
}

// line returns source line n (1-based) without its line break.
func (p *parser) line(n int) string {
	start := p.lines.starts[n-1]
	end := len(p.src)

	if n < len(p.lines.starts) {
		end = p.lines.starts[n]
	}

	return strings.TrimRight(string(p.src[start:end]), "\r\n")
}

// gluedMarkers returns the lines of a top-level blockquote, after its first, that start with a schedule marker.
func (p *parser) gluedMarkers(bq ast.Node) []int {
	from, to := p.span(bq)

	var out []int

	for n := from + 1; n <= to && from > 0; n++ {
		if gluedLine.MatchString(p.line(n)) {
			out = append(out, n)
		}
	}

	return out
}

// gluedMessage asks about a marker on line n inside the top-level blockquote bq, in plain words: did the author mean
// a blank line (new block) or a quoted line (same block) at the line that glued them? It returns "" when the marker
// isn't glued that way (a nested list, a bold marker, …).
func (p *parser) gluedMessage(bq ast.Node, n int) string {
	glued := false

	for _, g := range p.gluedMarkers(bq) {
		glued = glued || g == n
	}

	if !glued {
		return ""
	}

	c := p.culprit(bq, n)

	switch {
	case c == n-1:
		return `Did you mean to put a blank line above, or a line starting with "> " to continue the previous block?`
	case c > 0:
		return fmt.Sprintf(`Did you mean a blank line at line %d, or a line starting with "> " there to continue `+
			"the previous block?", c)
	default:
		return "Did you mean to put a blank line above? Without it, this block continues the previous one."
	}
}

// culprit returns the line that glued the marker on line n to the start of bq: the last line before it without ">"
// that isn't empty (a lazy continuation line); 0 when the blocks are glued only by missing empty lines.
func (p *parser) culprit(bq ast.Node, n int) int {
	from, _ := p.span(bq)

	for c := n - 1; c > from; c-- {
		if text := p.line(c); TrimSpace(text) != "" && !quotedLine.MatchString(text) {
			return c
		}
	}

	return 0
}

// cutLine is where a directive's blockquote stops counting as that directive: the culprit line of its first glued
// marker, else that marker's line; 0 when nothing is glued to it.
func (p *parser) cutLine(bq ast.Node) int {
	glued := p.gluedMarkers(bq)
	if len(glued) == 0 {
		return 0
	}

	if c := p.culprit(bq, glued[0]); c > 0 {
		return c
	}

	return glued[0]
}

// topLevel returns the ancestor of n that is a direct child of the document.
func topLevel(n ast.Node) ast.Node {
	for n.Parent() != nil && n.Parent().Kind() != ast.KindDocument {
		n = n.Parent()
	}

	return n
}
