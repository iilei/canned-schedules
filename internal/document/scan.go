package document

import (
	"regexp"

	"github.com/yuin/goldmark/ast"

	"github.com/iilei/canned-schedules/internal/diag"
)

// directiveLike matches text that looks like a directive marker: "[!" followed by a namespace and a colon.
var directiveLike = regexp.MustCompile(`\[!([A-Za-z][A-Za-z0-9-]*):`)

// scanMisplaced reports directive-like markers anywhere except on the marker line of a top-level directive, such as
// directives nested in lists or bold-wrapped markers. Code spans are blanked out first, so documentation that shows
// the syntax in backticks is fine.
func (p *parser) scanMisplaced(root ast.Node) {
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n.Kind() {
		case ast.KindParagraph, ast.KindTextBlock, ast.KindHeading:
			p.scanBlock(n)

			return ast.WalkSkipChildren, nil
		default:
			return ast.WalkContinue, nil
		}
	})
}

func (p *parser) scanBlock(n ast.Node) {
	masked := p.maskCode(n)
	lines := n.Lines()

	for i := range lines.Len() {
		seg := lines.At(i)
		if p.markerLines[seg.Start] {
			continue
		}

		for _, m := range directiveLike.FindAllIndex(masked[seg.Start:seg.Stop], -1) {
			pos := p.lines.pos(seg.Start + m[0] + len("[!"))
			p.errorf(pos, diag.CodeMisplacedDirective,
				"directives must be top-level blockquotes that start with the marker")
			p.example("> [!SCHEDULE:TIER] Medium")
		}
	}
}

// maskCode returns a copy of the source with the content of n's code spans replaced by spaces.
func (p *parser) maskCode(n ast.Node) []byte {
	masked := []byte(nil)

	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || c.Kind() != ast.KindCodeSpan {
			return ast.WalkContinue, nil
		}

		if masked == nil {
			masked = append([]byte(nil), p.src...)
		}

		for t := c.FirstChild(); t != nil; t = t.NextSibling() {
			if txt, ok := t.(*ast.Text); ok {
				for i := txt.Segment.Start; i < txt.Segment.Stop; i++ {
					masked[i] = ' '
				}
			}
		}

		return ast.WalkSkipChildren, nil
	})

	if masked == nil {
		return p.src
	}

	return masked
}
