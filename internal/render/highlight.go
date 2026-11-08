package render

import (
	"fmt"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/schedule"
)

// Highlight kinds.
const (
	HighlightRange = "range"
	HighlightBase  = "base"
)

// Highlight is the part of the SVG week one source entry paints: the preflight page stripes it while the editor
// cursor is in that entry. It uses the preview's buckets, so the stripes line up with the drawn segments.
type Highlight struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	Kind string `json:"kind"`
	// Line is the source line of the range item or the BASE block; 0 for a base from --base or the config.
	Line int `json:"line"`
	// Block is the line span the cursor may be in: the STATE block for a range, the BASE line for the base.
	Block schedule.Lines `json:"block"`
	State string         `json:"state"`
	Class string         `json:"class"`
	// Rects are the buckets where this entry shows; Shadowed those it covers mostly but a later range paints over.
	Rects    []Rect `json:"rects"`
	Shadowed []Rect `json:"shadowed"`
	// Label describes the entry in exact minutes, for a status line.
	Label string `json:"label"`
}

// Rect is a rectangle in the SVG's user coordinates.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Highlights computes one highlight per trace entry (base first, then the ranges in source order) on the buckets of
// p, the preview the SVG draws. A bucket is an entry's when it shows the entry's state and, among the minutes of that
// state, the entry paints the most (ties go to the later range, as in painting). A range's bucket is shadowed when the
// range covers at least half of it but the bucket isn't the range's.
func Highlights(tr *schedule.Trace, cfg *config.Config, p Preview) []Highlight {
	winners := bucketWinners(tr, p)
	lines := tr.Lines()
	out := make([]Highlight, 0, len(tr.Ranges)+1)

	for o := -1; o < len(tr.Ranges); o++ {
		i, _ := stateInfo(cfg, tr.State(o))
		h := Highlight{
			Kind: HighlightRange, State: tr.State(o), Class: stateClass(i), Rects: []Rect{},
			Shadowed: []Rect{},
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
