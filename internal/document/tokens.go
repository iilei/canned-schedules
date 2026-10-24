package document

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
)

// tokens flattens the inline content of a list item into code, text and other tokens.
func (p *parser) tokens(block ast.Node) []Token {
	var out []Token

	for c := block.FirstChild(); c != nil; c = c.NextSibling() {
		switch n := c.(type) {
		case *ast.CodeSpan:
			out = append(out, p.codeToken(n))
		case *ast.Text:
			value := string(n.Segment.Value(p.src))
			if n.SoftLineBreak() || n.HardLineBreak() {
				value += "\n"
			}

			if last := len(out) - 1; last >= 0 && out[last].Kind == TokenText {
				out[last].Value += value

				continue
			}

			out = append(out, Token{Kind: TokenText, Value: value, Pos: p.lines.pos(n.Segment.Start)})
		default:
			out = append(out, Token{Kind: TokenOther, Pos: p.blockPos(n)})
		}
	}

	return out
}

func (p *parser) codeToken(n *ast.CodeSpan) Token {
	var (
		value bytes.Buffer
		start = -1
	)

	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			if start < 0 {
				start = t.Segment.Start
			}

			value.Write(t.Segment.Value(p.src))
		}
	}

	tok := Token{Kind: TokenCode, Value: value.String()}
	if start >= 0 {
		tok.Pos = p.lines.pos(start)
	}

	return tok
}
