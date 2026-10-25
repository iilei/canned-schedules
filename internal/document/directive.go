package document

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"

	"github.com/iilei/canned-schedules/internal/diag"
)

// Namespaces lists the accepted namespaces, matched case-insensitively. A directive's alert type is the namespace, a
// hyphen and the kind ([!SCHEDULE-STATE]): the hyphen keeps the markers valid callout types for other tools, such as
// rehype-callouts. There is one namespace for now; it stays a list so an alias can be added later. Adding one is a
// spec change.
var Namespaces = []string{"SCHEDULE"}

// marker matches the alert marker at the start of a blockquote's first line and captures the alert type.
var marker = regexp.MustCompile(`^\[!([^\]\n]*)\]`)

var kinds = map[string]Kind{"META": KindMeta, "BASE": KindBase, "STATE": KindState}

// blockquote handles a top-level blockquote. Blockquotes that aren't directives (plain quotes, NOTE, TIP, …) are
// left alone; scanMisplaced still looks into them.
func (p *parser) blockquote(bq ast.Node) {
	para, ok := bq.FirstChild().(*ast.Paragraph)
	if !ok || para.Lines().Len() == 0 {
		return
	}

	first := para.Lines().At(0)
	line := strings.TrimRight(string(first.Value(p.src)), "\r\n")

	m := marker.FindStringSubmatchIndex(line)
	if m == nil {
		return
	}

	typ := line[m[2]:m[3]]
	kindPos := p.lines.pos(first.Start + m[2])

	if ns, _, ok := strings.Cut(typ, ":"); ok && isNamespace(ns) {
		p.errorf(kindPos, diag.CodeInvalidMarker, "[!%s]: directives separate the namespace and the kind with a hyphen",
			typ)
		p.example("> [!SCHEDULE-STATE] Medium")

		return
	}

	kindName, ours := SplitMarker(typ)
	if !ours {
		// Any other alert (NOTE, TIP, custom callouts) is prose for humans, unless --error-unless-alert-pattern
		// restricts which ones may appear.
		if pat := p.opts.AlertPattern; pat != nil && !pat.MatchString(typ) {
			p.errorf(kindPos, diag.CodeDisallowedAlert, "alert [!%s] doesn't match --error-unless-alert-pattern", typ)
		}

		return
	}

	p.markerLines[first.Start] = true

	kind, ok := p.kind(typ, kindName, kindPos)
	if !ok {
		return
	}

	rest := line[m[1]:]
	lead := len(rest) - len(TrimLeftSpace(rest))
	d := Directive{
		Kind:    kind,
		KindPos: kindPos,
		Title:   Text{Value: TrimSpace(rest), Pos: p.lines.pos(first.Start + m[1] + lead)},
	}

	hasBody := p.body(&d, para)
	p.add(&d, hasBody)
}

func (p *parser) kind(typ, kindName string, pos diag.Pos) (Kind, bool) {
	if kindName == "" {
		p.errorf(pos, diag.CodeMissingKind, "[!%s] needs a kind: META, BASE or STATE", typ)
		p.example("> [!SCHEDULE-STATE] Medium")

		return "", false
	}

	if k, ok := kinds[strings.ToUpper(kindName)]; ok {
		return k, true
	}

	p.errorf(pos, diag.CodeUnknownKind, "unknown kind %q in [!%s]; use META, BASE or STATE", kindName, typ)
	p.example("> [!SCHEDULE-STATE] Medium")

	return "", false
}

// SplitMarker reports whether an alert type is a schedule directive ([!SCHEDULE-STATE], also the bare [!SCHEDULE])
// and returns its kind as written, empty when missing.
func SplitMarker(typ string) (string, bool) {
	for _, ns := range Namespaces {
		if len(typ) < len(ns) || !strings.EqualFold(typ[:len(ns)], ns) {
			continue
		}

		switch rest := typ[len(ns):]; {
		case rest == "":
			return "", true
		case rest[0] == '-':
			return rest[1:], true
		}
	}

	return "", false
}

// body collects the list items of a directive and reports any other content. It returns whether the directive has
// any content besides its marker line.
func (p *parser) body(d *Directive, para *ast.Paragraph) bool {
	hasBody := false

	for i := 1; i < para.Lines().Len(); i++ {
		seg := para.Lines().At(i)
		raw := string(seg.Value(p.src))
		lead := len(raw) - len(TrimLeftSpace(raw))
		p.unexpected(d, p.lines.pos(seg.Start+lead))

		hasBody = true
	}

	for n := para.NextSibling(); n != nil; n = n.NextSibling() {
		hasBody = true

		list, ok := n.(*ast.List)
		if !ok || list.IsOrdered() || d.Kind == KindBase {
			p.unexpected(d, p.blockPos(n))

			continue
		}

		for li := list.FirstChild(); li != nil; li = li.NextSibling() {
			p.item(d, li)
		}
	}

	return hasBody
}

func (p *parser) item(d *Directive, li ast.Node) {
	content := li.FirstChild()
	if content == nil || (content.Kind() != ast.KindTextBlock && content.Kind() != ast.KindParagraph) {
		pos := d.KindPos
		if content != nil {
			pos = p.blockPos(content)
		}

		p.unexpected(d, pos)

		return
	}

	for extra := content.NextSibling(); extra != nil; extra = extra.NextSibling() {
		p.unexpected(d, p.blockPos(extra))
	}

	d.Items = append(d.Items, Item{Pos: p.blockPos(content), Tokens: p.tokens(content)})
}

// unexpected reports content that doesn't belong in a directive.
func (p *parser) unexpected(d *Directive, pos diag.Pos) {
	switch d.Kind {
	case KindState:
		p.errorf(pos, diag.CodeInvalidRange, "a STATE block holds a list of ranges")
		p.example("- `06:00` to `22:00` MON-FRI")
	case KindMeta:
		p.errorf(pos, diag.CodeMetaSchema, "a META block holds a list of key: value items")
		p.example("- x-owner: `team-xyz`")
	case KindBase:
		p.errorf(pos, diag.CodeUnexpectedContent, "BASE takes only a state name on the marker line")
	}
}

// add stores a directive after the per-kind checks.
func (p *parser) add(d *Directive, hasBody bool) {
	switch d.Kind {
	case KindMeta:
		if p.duplicate(p.doc.Meta, d) {
			return
		}

		if d.Title.Value != "" && !textRule.MatchString(d.Title.Value) {
			p.errorf(d.Title.Pos, diag.CodeInvalidHint,
				"hint %q: 1-80 ASCII letters, digits, spaces or hyphens; starting with a letter, ending with a letter "+
					"or digit", d.Title.Value)
		}

		p.doc.Meta = d
	case KindBase:
		if p.duplicate(p.doc.Base, d) || !p.titled(d) {
			return
		}

		p.doc.Base = d
	case KindState:
		if !p.titled(d) {
			return
		}

		if !hasBody {
			p.errorf(d.KindPos, diag.CodeEmptyState, "STATE %s has no ranges", d.Title.Value)

			return
		}

		p.doc.States = append(p.doc.States, *d)
	}
}

func (p *parser) duplicate(prev, d *Directive) bool {
	if prev == nil {
		return false
	}

	p.errorf(d.KindPos, diag.CodeDuplicateBlock, "only one %s block is allowed (the first one is at line %d)",
		d.Kind, prev.KindPos.Line)

	return true
}

func (p *parser) titled(d *Directive) bool {
	if d.Title.Value != "" {
		return true
	}

	p.errorf(d.KindPos, diag.CodeMissingTitle, "%s needs a state name after the marker", d.Kind)
	p.example("> [!SCHEDULE-" + string(d.Kind) + "] Medium")

	return false
}

func isNamespace(ns string) bool {
	for _, n := range Namespaces {
		if strings.EqualFold(n, ns) {
			return true
		}
	}

	return false
}
