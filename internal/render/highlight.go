package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/schedule"
)

// Highlight kinds.
const (
	HighlightName = "name"
	// HighlightMetaBlock is the whole META block, from its marker line: summary, annotations and schema-version.
	HighlightMetaBlock = "meta-block"
	HighlightMeta      = "meta" // A META item: schema-version or an x-* key.
	HighlightBase      = "base"
	HighlightRange     = "range"
)

// schemaVersionKey is the META key that isn't an annotation.
const schemaVersionKey = "schema-version"

// Text box padding around a label, and the font sizes the template uses.
const (
	boxPad        = 3
	titleFontSize = 18
	headerFont    = 14
	bodyFont      = 12
	boxDescent    = 0.35 // Of the font size, below the baseline.
	rowBoxGap     = 2    // Neighbouring annotation rows' frames stay this far apart.
	rowBoxRaise   = 0.5  // Row frames start this far above the font size over the baseline.
)

// Highlight is the part of the SVG week one source entry paints: the preflight page stripes it while the editor
// cursor is in that entry. It uses the preview's buckets, so the stripes line up with the drawn segments.
type Highlight struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	Kind string `json:"kind"`
	// Line is the source line of the entry: the H1, the META marker (summary), a META item, the BASE block or a range
	// item; 0 for a base from --base or the config.
	Line int `json:"line"`
	// Block is the block the entry belongs to: the META block for the summary and META items, the STATE block for a
	// range, the entry's own line otherwise. The cursor on a block's first line highlights all its entries.
	Block schedule.Lines `json:"block"`
	// Key is the META item's key (schema-version or x-*); empty for other kinds.
	Key   string `json:"key,omitempty"`
	State string `json:"state,omitempty"`
	Class string `json:"class,omitempty"`
	// Rects are the buckets where this entry shows; Shadowed those it covers mostly but a later range paints over.
	Rects    []Rect `json:"rects"`
	Shadowed []Rect `json:"shadowed"`
	// Label describes the entry (in exact minutes for base and ranges), for a status line.
	Label string `json:"label"`
	// JSON are the line spans of the output JSON (as the page shows it) that the entry produces; filled in by the
	// page's evaluation, which owns the JSON text.
	JSON []schedule.Lines `json:"json"`
	// Owner is the painting owner for base (-1) and ranges (their index); unused for other kinds.
	Owner int `json:"-"`
}

// Rect is a rectangle in the SVG's user coordinates.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Highlights computes one highlight per trace entry: the name, the summary and the META items (boxes around their
// text in m, the model the SVG draws), then the base and the ranges in source order on the buckets of p. A bucket is an entry's when it shows the entry's state and, among the minutes of that
// state, the entry paints the most (ties go to the later range, as in painting). A range's bucket is shadowed when the
// range covers at least half of it but the bucket isn't the range's.
func Highlights(tr *schedule.Trace, cfg *config.Config, p Preview, m *Model) []Highlight {
	out := headerHighlights(tr, m)
	winners := bucketWinners(tr, p)
	lines := tr.Lines()

	for o := -1; o < len(tr.Ranges); o++ {
		i, _ := stateInfo(cfg, tr.State(o))
		h := Highlight{
			Kind: HighlightRange, State: tr.State(o), Class: stateClass(i), Rects: []Rect{},
			Shadowed: []Rect{}, Owner: o,
		}

		if o < 0 {
			h.Kind, h.Line = HighlightBase, tr.Base.Line
			if h.Line > 0 {
				h.Block = schedule.Lines{From: h.Line, To: h.Line}
			}
		} else {
			h.Line, h.Block = tr.Ranges[o].Line, tr.Ranges[o].Block
		}

		partial := false

		for d := range days {
			own, shadowed := make([]bool, len(winners[d])), make([]bool, len(winners[d]))

			for b, w := range winners[d] {
				n := coverage(tr, o, d*minutesPerDay+b*p.Size, p.Size)
				own[b] = w == o
				shadowed[b] = o >= 0 && !own[b] && 2*n >= p.Size
				partial = partial || (n > 0 && n < p.Size)
			}

			h.Rects = append(h.Rects, bucketRects(own, d, p.Size)...)
			h.Shadowed = append(h.Shadowed, bucketRects(shadowed, d, p.Size)...)
		}

		h.Label = lines[o+1].Message
		if h.Line > 0 {
			h.Label = fmt.Sprintf("line %d: %s", h.Line, h.Label)
		}

		if partial && p.Size > 1 {
			h.Label += fmt.Sprintf(" (stripes in %d-minute buckets)", p.Size)
		}

		out = append(out, h)
	}

	return out
}

// headerHighlights covers what the H1 and the META block produce: name, summary and the META items. Their SVG
// rects frame the title, the summary and (with --svg-annotations) the annotation rows.
func headerHighlights(tr *schedule.Trace, m *Model) []Highlight {
	line := func(n int) schedule.Lines { return schedule.Lines{From: n, To: n} }
	h := Highlight{
		Kind: HighlightName, Line: tr.Name.Line, Block: line(tr.Name.Line), Rects: []Rect{}, Shadowed: []Rect{},
		Label: fmt.Sprintf("line %d: name %q", tr.Name.Line, tr.Name.Value),
	}

	if m.Title.Text != "" {
		h.Rects = append(h.Rects, textBox(m.Title, titleCharW, titleFontSize))
	}

	out := []Highlight{h}

	if tr.Meta == nil {
		return out
	}

	out = append(out, metaBlock(tr.Meta, m))

	for _, e := range tr.Meta.Entries {
		h := Highlight{
			Kind: HighlightMeta, Line: e.Line, Block: tr.Meta.Block, Key: e.Key, Rects: []Rect{}, Shadowed: []Rect{},
			Label: fmt.Sprintf("line %d: %s %q", e.Line, e.Key, e.Value),
		}

		if m.Annotations != nil {
			for _, r := range m.Annotations.Rows {
				if r.Key.Text == e.Key {
					box := textBox(r.Key, charW, bodyFont)
					box.Width = r.Value.X - r.Key.X + float64(len([]rune(r.Value.Text)))*charW + boxPad + boxPad
					box.Y, box.Height = r.Key.Y-bodyFont-rowBoxRaise, lineH-rowBoxGap
					h.Rects = append(h.Rects, box)
				}
			}
		}

		out = append(out, h)
	}

	return out
}

// metaBlock is the entry of the META marker line: everything the block produces. Its SVG rects frame the summary and
// the whole annotations section; its JSON spans (filled in by the page) cover summary, annotations and
// schema-version.
func metaBlock(meta *schedule.TraceMeta, m *Model) Highlight {
	h := Highlight{
		Kind: HighlightMetaBlock, Line: meta.Block.From, Block: meta.Block, Rects: []Rect{}, Shadowed: []Rect{},
	}

	var parts []string

	if s := meta.Summary; s != nil {
		parts = append(parts, fmt.Sprintf("summary %q", s.Value))

		if m.Summary.Text != "" {
			h.Rects = append(h.Rects, textBox(m.Summary, headerCharW, headerFont))
		}
	}

	annotations := 0

	for _, e := range meta.Entries {
		if e.Key == schemaVersionKey {
			parts = append(parts, fmt.Sprintf("schema-version %q", e.Value))
		} else {
			annotations++
		}
	}

	if annotations > 0 {
		parts = append(parts, fmt.Sprintf("%d annotation(s)", annotations))
	}

	if a := m.Annotations; a != nil && len(a.Rows) > 0 {
		box := textBox(a.Heading, headerCharW, headerFont)
		last := a.Rows[len(a.Rows)-1]

		for _, r := range a.Rows {
			box.Width = math.Max(box.Width, r.Value.X-box.X+float64(len([]rune(r.Value.Text)))*charW+boxPad)
		}

		box.Height = last.Key.Y + bodyFont*boxDescent + boxPad - box.Y
		h.Rects = append(h.Rects, box)
	}

	h.Label = fmt.Sprintf("line %d: META block", h.Line)
	if len(parts) > 0 {
		h.Label += ": " + strings.Join(parts, ", ")
	}

	return h
}

// textBox frames a label drawn at t with the given character advance and font size.
func textBox(t Text, advance, size float64) Rect {
	return Rect{
		X: t.X - boxPad, Y: t.Y - size,
		Width:  float64(len([]rune(t.Text)))*advance + 2*boxPad,
		Height: size*(1+boxDescent) + boxPad,
	}
}

// bucketWinners returns, per day and bucket, the entry (-1 = base) whose minutes the bucket shows: among the minutes
// in the bucket's shown state, the entry painting the most, ties to the later one.
func bucketWinners(tr *schedule.Trace, p Preview) [][]int {
	out := make([][]int, len(days))

	for d := range days {
		out[d] = make([]int, len(p.Days[d]))

		for b, shown := range p.Days[d] {
			counts := map[int]int{}
			start := d*minutesPerDay + b*p.Size

			for m := start; m < start+p.Size; m++ {
				if o := tr.Owner(m); tr.State(o) == shown {
					counts[o]++
				}
			}

			best, bestN := -1, -1
			for o, n := range counts {
				if n > bestN || (n == bestN && o > best) {
					best, bestN = o, n
				}
			}

			out[d][b] = best
		}
	}

	return out
}

// coverage counts the minutes in [start, start+size) that entry o covers; the base covers what no range paints.
func coverage(tr *schedule.Trace, o, start, size int) int {
	n := 0

	for m := start; m < start+size; m++ {
		if (o < 0 && tr.Owner(m) < 0) || (o >= 0 && tr.Covers(o, m)) {
			n++
		}
	}

	return n
}

// bucketRects merges marked neighbouring buckets of day d into rectangles on the SVG week.
func bucketRects(marked []bool, d, size int) []Rect {
	var out []Rect

	y := float64(titleH + rulerH + d*dayH)

	for b := 0; b < len(marked); b++ {
		if !marked[b] {
			continue
		}

		e := b
		for e+1 < len(marked) && marked[e+1] {
			e++
		}

		out = append(out, Rect{X: labelW + px(b*size), Y: y, Width: px((e - b + 1) * size), Height: blockH})
		b = e
	}

	return out
}
