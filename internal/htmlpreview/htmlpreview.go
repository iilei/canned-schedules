// Package htmlpreview renders a schedule document as HTML, either the way GitHub shows an issue or the way a Sphinx
// handbook with custom admonitions would. It is a preview: raw HTML in the document is never passed through, and
// dangerous link targets are dropped (goldmark's safe defaults).
package htmlpreview

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/document"
)

// Modes.
const (
	// GitHub renders standard alerts (NOTE, TIP, …) as GitHub callouts and leaves every other marker as a plain
	// quote with the literal text, which is what GitHub shows for schedule directives.
	GitHub = "github"
	// Sphinx renders every alert as an admonition; schedule directives get custom admonitions coloured from the
	// config's tier styles, and weekday names get <abbr> tooltips.
	Sphinx = "sphinx"
)

const transformerPriority = 100

// standardAlerts are the alert kinds GitHub (and MyST's alert extension) know, with their titles.
var standardAlerts = map[string]string{
	"NOTE": "Note", "TIP": "Tip", "IMPORTANT": "Important", "WARNING": "Warning", "CAUTION": "Caution",
}

var (
	marker  = regexp.MustCompile(`^\[!([^\]\n]*)\][ \t]*(.*)$`)
	weekday = regexp.MustCompile(`\b(MON|TUE|WED|THU|FRI|SAT|SUN)\b`)
)

var weekdayNames = map[string]string{
	"MON": "Monday", "TUE": "Tuesday", "WED": "Wednesday", "THU": "Thursday", "FRI": "Friday", "SAT": "Saturday",
	"SUN": "Sunday",
}

// kindCallout identifies Callout nodes.
var kindCallout = ast.NewNodeKind("Callout")

// Callout is a blockquote that starts with an alert marker. Its children are the blockquote's content without the
// marker line.
type Callout struct {
	// Class is the CSS class list.
	Class string
	// Title is the visible title.
	Title string
	ast.BaseBlock
	// Schedule marks schedule directives, whose content gets weekday tooltips in Sphinx mode.
	Schedule bool
}

// transformer replaces top-level marker blockquotes with Callout nodes.
type transformer struct {
	cfg  *config.Config
	mode string
}

// calloutRenderer renders Callout nodes.
type calloutRenderer struct {
	mode string
}

// Kind implements ast.Node.
func (*Callout) Kind() ast.NodeKind { return kindCallout }

// Dump implements ast.Node.
func (n *Callout) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Class": n.Class, "Title": n.Title}, nil)
}

// Render renders src in the given mode, with tier names and classes from cfg.
func Render(src []byte, cfg *config.Config, mode string) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithASTTransformers(
			util.Prioritized(&transformer{cfg: cfg, mode: mode}, transformerPriority))),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(
			util.Prioritized(&calloutRenderer{mode: mode}, transformerPriority))),
	)

	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return "", fmt.Errorf("render preview: %w", err)
	}

	if mode == Sphinx {
		return abbrWeekdays(buf.String()), nil
	}

	return buf.String(), nil
}

// Transform implements parser.ASTTransformer.
func (t *transformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()

	var next ast.Node
	for n := doc.FirstChild(); n != nil; n = next {
		next = n.NextSibling()

		if bq, ok := n.(*ast.Blockquote); ok {
			if c := t.callout(bq, src); c != nil {
				doc.ReplaceChild(doc, bq, c)
			}
		}
	}
}

func (t *transformer) callout(bq *ast.Blockquote, src []byte) *Callout {
	para, ok := bq.FirstChild().(*ast.Paragraph)
	if !ok || para.Lines().Len() == 0 {
		return nil
	}

	first := para.Lines().At(0)
	m := marker.FindStringSubmatch(strings.TrimRight(string(first.Value(src)), "\r\n"))

	if m == nil {
		return nil
	}

	c := t.classify(m[1], strings.TrimSpace(m[2]))
	if c == nil {
		return nil
	}

	dropFirstLine(para, first.Stop)

	if para.ChildCount() == 0 {
		bq.RemoveChild(bq, para)
	}

	for child := bq.FirstChild(); child != nil; child = bq.FirstChild() {
		c.AppendChild(c, child)
	}

	return c
}

// classify decides how a marker renders; nil leaves the blockquote alone.
func (t *transformer) classify(typ, title string) *Callout {
	if std, ok := standardAlerts[strings.ToUpper(typ)]; ok {
		if t.mode == GitHub && title != "" {
			return nil // GitHub only recognises a marker that stands alone on its line.
		}

		class := "markdown-alert markdown-alert-" + strings.ToLower(typ)
		if t.mode == Sphinx {
			class = "admonition " + strings.ToLower(typ)
		}

		return &Callout{Class: class, Title: std}
	}

	if t.mode != Sphinx {
		return nil
	}

	ns, kind, ok := strings.Cut(typ, ":")
	if !ok || !isNamespace(ns) {
		return &Callout{Class: "admonition", Title: strings.TrimSpace(typ + " " + title)}
	}

	return t.schedule(strings.ToUpper(kind), title)
}

func (t *transformer) schedule(kind, title string) *Callout {
	c := &Callout{Schedule: true}

	switch kind {
	case string(document.KindMeta):
		c.Class, c.Title = "admonition schedule schedule-meta", "Schedule"
		if title != "" {
			c.Title += ": " + title
		}
	case string(document.KindBase):
		c.Class, c.Title = "admonition schedule schedule-base "+t.tierClass(title), "Base tier: "+t.tierName(title)
	case string(document.KindTier):
		c.Class, c.Title = "admonition schedule schedule-tier "+t.tierClass(title), t.tierName(title)
	default:
		c.Class, c.Title = "admonition schedule", kind+" "+title
	}

	return c
}

func (t *transformer) tierClass(id string) string {
	for i, tier := range t.cfg.Tiers {
		if strings.EqualFold(tier.ID, id) {
			return fmt.Sprintf("tier-%d", i)
		}
	}

	return "tier-unknown"
}

func (t *transformer) tierName(id string) string {
	if tier, ok := t.cfg.Tier(id); ok {
		return tier.DisplayName
	}

	return id
}

// dropFirstLine removes the paragraph's inline content up to the end of its first line (the marker).
func dropFirstLine(para *ast.Paragraph, stop int) {
	for c := para.FirstChild(); c != nil; c = para.FirstChild() {
		start, ok := firstOffset(c)
		if !ok || start >= stop {
			return
		}

		para.RemoveChild(para, c)
	}
}

func firstOffset(n ast.Node) (int, bool) {
	if t, ok := n.(*ast.Text); ok {
		return t.Segment.Start, true
	}

	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if off, ok := firstOffset(c); ok {
			return off, true
		}
	}

	return 0, false
}

func isNamespace(ns string) bool {
	for _, n := range document.Namespaces {
		if strings.EqualFold(n, ns) {
			return true
		}
	}

	return false
}

// RegisterFuncs implements renderer.NodeRenderer.
func (r *calloutRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindCallout, r.render)
}

func (r *calloutRenderer) render(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	c, ok := n.(*Callout)
	if !ok {
		return ast.WalkContinue, nil
	}

	titleClass := "admonition-title"
	if r.mode == GitHub {
		titleClass = "markdown-alert-title"
	}

	if entering {
		_, _ = fmt.Fprintf(w, `<div class="%s"><p class="%s">%s</p>`+"\n",
			util.EscapeHTML([]byte(c.Class)), titleClass, util.EscapeHTML([]byte(c.Title)))

		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString("</div>\n")

	return ast.WalkContinue, nil
}
