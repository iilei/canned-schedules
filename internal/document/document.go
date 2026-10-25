// Package document parses a schedule document into its syntax: the name, and the META, BASE and STATE directives
// with positions. It knows nothing about the config, states or time expressions.
package document

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/iilei/canned-schedules/internal/diag"
)

// Directive kinds.
const (
	KindMeta  Kind = "META"
	KindBase  Kind = "BASE"
	KindState Kind = "STATE"
)

// Token kinds.
const (
	// TokenText is plain text. Adjacent text nodes are merged; soft line breaks become "\n".
	TokenText TokenKind = iota
	// TokenCode is the content of a code span, without the backticks.
	TokenCode
	// TokenOther is any other inline element (emphasis, links, …). Its Value is empty.
	TokenOther
)

// textRule is the shared rule for the name and the hint: 1–80 ASCII chars, a letter first, letters, digits, spaces
// and hyphens in between, and a letter or digit last.
var textRule = regexp.MustCompile(`^[A-Za-z](?:[A-Za-z0-9 -]{0,78}[A-Za-z0-9])?$`)

// Kind is the kind of a directive.
type Kind string

// TokenKind tells code spans from plain text in a list item.
type TokenKind int

// Text is a piece of source text with the position of its first character.
type Text struct {
	Value string
	Pos   diag.Pos
}

// Token is one element of a list item.
type Token struct {
	Value string
	Pos   diag.Pos
	Kind  TokenKind
}

// Item is one list item of a directive. Pos is the first character of the item's content.
type Item struct {
	Tokens []Token
	Pos    diag.Pos
}

// Directive is a top-level `> [!NS:KIND] Title` block.
type Directive struct {
	Items   []Item
	Kind    Kind
	Title   Text
	KindPos diag.Pos
}

// Document is the parsed syntax of a schedule document.
type Document struct {
	States []Directive
	Meta   *Directive
	Base   *Directive
	Name   Text
}

// Options configure Parse.
type Options struct {
	// AlertPattern is --error-unless-alert-pattern, anchored: when set, every alert that isn't a schedule directive
	// must match it, otherwise it is a disallowed-alert error. Nil allows any alert.
	AlertPattern *regexp.Regexp
}

type parser struct {
	markerLines map[int]bool
	opts        Options
	src         []byte
	lines       lineIndex
	diags       diag.List
	doc         Document
}

// Parse parses src. Diagnostics are sorted by position, those without a position first.
func Parse(src []byte, opts Options) (*Document, diag.List) {
	p := &parser{src: src, opts: opts, lines: newLineIndex(src), markerLines: map[int]bool{}}
	root := goldmark.New().Parser().Parse(text.NewReader(src))

	var nameSeen bool

	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		switch n.Kind() {
		case ast.KindHeading:
			if h, ok := n.(*ast.Heading); ok && h.Level == 1 {
				p.name(h, nameSeen)
				nameSeen = true
			}
		case ast.KindBlockquote:
			p.blockquote(n)
		default:
		}
	}

	if !nameSeen {
		p.errorf(diag.Pos{}, diag.CodeMissingName, "the document needs an H1 heading with the schedule name")
		p.example("# Office Hours")
	}

	p.scanMisplaced(root)

	p.diags.Sort()

	return &p.doc, p.diags
}

func (p *parser) errorf(pos diag.Pos, code, format string, args ...any) {
	p.diags = append(p.diags, diag.Errorf(pos, code, format, args...))
}

// example adds an example to the diagnostic reported last.
func (p *parser) example(example string) {
	p.diags[len(p.diags)-1].Example = example
}

func (p *parser) name(h *ast.Heading, seen bool) {
	t := p.blockText(h)
	if seen {
		p.errorf(t.Pos, diag.CodeDuplicateName, "only one H1 heading is allowed; the first one is the schedule name")

		return
	}

	p.doc.Name = t
	if !textRule.MatchString(t.Value) {
		p.errorf(
			t.Pos,
			diag.CodeInvalidName,
			"name %q: 1-80 ASCII letters, digits, spaces or hyphens; starting with a letter, ending with a letter or digit",
			t.Value,
		)
	}
}

// blockText returns the raw text of a block's lines, trimmed, with the position of its first character.
func (p *parser) blockText(n ast.Node) Text {
	lines := n.Lines()
	if lines.Len() == 0 {
		return Text{}
	}

	var b strings.Builder
	for i := range lines.Len() {
		seg := lines.At(i)
		b.Write(seg.Value(p.src))
	}

	raw := b.String()
	trimmed := strings.TrimSpace(raw)
	lead := len(raw) - len(strings.TrimLeft(raw, " \t"))

	return Text{Value: trimmed, Pos: p.lines.pos(lines.At(0).Start + lead)}
}

// blockPos returns the position of the first source character of n or of its first descendant that has one.
func (p *parser) blockPos(n ast.Node) diag.Pos {
	if off, ok := firstOffset(n); ok {
		return p.lines.pos(off)
	}

	return diag.Pos{}
}

func firstOffset(n ast.Node) (int, bool) {
	if t, ok := n.(*ast.Text); ok {
		return t.Segment.Start, true
	}

	if n.Type() == ast.TypeBlock && n.Lines().Len() > 0 {
		return n.Lines().At(0).Start, true
	}

	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if off, ok := firstOffset(c); ok {
			return off, true
		}
	}

	return 0, false
}
