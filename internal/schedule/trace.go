package schedule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/document"
	"github.com/iilei/canned-schedules/internal/expr"
	"github.com/iilei/canned-schedules/schema"
)

// Base origins in a trace: where the base state came from.
const (
	BaseFromDocument = "document"
	BaseFromFlag     = "flag"
	BaseFromConfig   = "config"
)

const minutesPerWeek = 7 * minutesPerDay

var traceSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	return schema.Compile(schema.TraceSchemaURL, schema.TraceSchema)
})

// Trace explains the painting: which minutes each range covers, where it wins and where later ranges paint over it.
// It is a debugging aid (--verbose, --trace-output-file, the page's highlights), outside the stability promise: its
// shape may change without a new schema-version. Minutes are exact, never bucketed. Field order is the JSON key order.
type Trace struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	// Name is the H1; Meta the META block, absent without one. They say where the output's name, summary,
	// schema-version and annotations come from.
	Name   TraceText    `json:"name"`
	Meta   *TraceMeta   `json:"meta,omitempty"`
	Base   TraceBase    `json:"base"`
	Ranges []TraceRange `json:"ranges"`
	// owners holds, per minute of the week from MON 00:00, the index of the range that paints it last; -1 is base.
	owners []int
	ranges []stateRange
}

// TraceText is a piece of source text with its position.
type TraceText struct {
	Value  string `json:"value"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// TraceMeta is the META block: its lines, the summary on its marker line and its items in source order.
type TraceMeta struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	Block   Lines        `json:"block"`
	Summary *TraceText   `json:"summary,omitempty"`
	Entries []TraceEntry `json:"entries"`
}

// TraceEntry is one META item: schema-version or an x-* key, which the output lists under annotations.
type TraceEntry struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// TraceBase is the base state and the minutes no range paints over.
type TraceBase struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	State string `json:"state"`
	// Origin is BaseFromDocument (the BASE block), BaseFromFlag (--base) or BaseFromConfig (the first state).
	Origin    string `json:"origin"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	Minutes   int    `json:"minutes"`
	Effective []Span `json:"effective"`
}

// TraceRange is one range item, in source order.
type TraceRange struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	Line   int `json:"line"`
	Column int `json:"column"`
	// Block is the line span of the STATE block the item belongs to, from its marker to its last item.
	Block     Lines          `json:"block"`
	State     string         `json:"state"`
	Start     string         `json:"start"`
	End       string         `json:"end"`
	Days      []string       `json:"days"`
	Minutes   int            `json:"minutes"`
	Covers    []Span         `json:"covers"`
	Effective []Span         `json:"effective"`
	Shadowed  []ShadowedSpan `json:"shadowed"`
}

// Lines is an inclusive span of source lines.
type Lines struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Span is a half-open [Start, End) stretch of one day. End "00:00" means the end of the day.
type Span struct {
	Day   string `json:"day"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// ShadowedSpan is a span a range covers but a later range paints over; By is that range's line.
type ShadowedSpan struct {
	Span

	By int `json:"by"`
}

type keyedSpan struct {
	Span

	key int
}

// TraceLine is one line of Trace.Lines.
type TraceLine struct {
	Message string
	Pos     diag.Pos
}

// Explain parses src and traces the painting. The trace is nil when there are errors.
func Explain(src []byte, cfg *config.Config, opts Options) (*Trace, diag.List) {
	ev, diags := evaluate(src, cfg, opts)
	if ev == nil {
		return nil, diags
	}

	return newTrace(ev, opts), diags
}

// Owner returns the index of the range that paints minute m of the week (0 = MON 00:00) last; -1 means base.
func (t *Trace) Owner(m int) int { return t.owners[m] }

// Covers reports whether range i covers minute m of the week, whether or not a later range paints over it.
func (t *Trace) Covers(i, m int) bool { return t.ranges[i].covers(m) }

// State returns the state id that owner paints; -1 is the base.
func (t *Trace) State(owner int) string {
	if owner < 0 {
		return t.Base.State
	}

	return t.Ranges[owner].State
}

func newTrace(ev *evaluation, opts Options) *Trace {
	t := &Trace{owners: paintOwners(ev.ranges), ranges: ev.ranges, Ranges: []TraceRange{}}

	t.Base = TraceBase{State: ev.base.ID, Origin: BaseFromConfig}
	t.Name = TraceText{Value: ev.doc.Name.Value, Line: ev.doc.Name.Pos.Line, Column: ev.doc.Name.Pos.Col}

	if d := ev.doc.Meta; d != nil {
		t.Meta = &TraceMeta{Block: blockLines(d), Entries: []TraceEntry{}}
		if d.Title.Value != "" {
			t.Meta.Summary = &TraceText{Value: d.Title.Value, Line: d.Title.Pos.Line, Column: d.Title.Pos.Col}
		}

		for _, e := range ev.meta.entries {
			t.Meta.Entries = append(t.Meta.Entries, TraceEntry{
				Key: e.key, Value: e.value, Line: e.keyPos.Line, Column: e.keyPos.Col,
			})
		}
	}

	switch {
	case ev.doc.Base != nil:
		t.Base.Origin, t.Base.Line, t.Base.Column = BaseFromDocument, ev.doc.Base.KindPos.Line, ev.doc.Base.KindPos.Col
	case opts.Base != "":
		t.Base.Origin = BaseFromFlag
	}

	t.Base.Effective, t.Base.Minutes = spans(func(m int) (int, bool) { return 0, t.owners[m] < 0 })

	for i, r := range ev.ranges {
		tr := TraceRange{
			Line: r.pos.Line, Column: r.pos.Col, Block: r.block, State: r.state.ID,
			Start: clock(r.Start), End: clock(r.End), Days: dayNames(r.Days),
		}

		tr.Covers, tr.Minutes = spans(func(m int) (int, bool) { return 0, r.covers(m) })
		tr.Effective, _ = spans(func(m int) (int, bool) { return 0, t.owners[m] == i })

		shadowed, _ := keyedSpans(func(m int) (int, bool) {
			o := t.owners[m]

			return o, o != i && r.covers(m)
		})

		tr.Shadowed = make([]ShadowedSpan, 0, len(shadowed))
		for _, s := range shadowed {
			tr.Shadowed = append(tr.Shadowed, ShadowedSpan{Span: s.Span, By: ev.ranges[s.key].pos.Line})
		}

		t.Ranges = append(t.Ranges, tr)
	}

	return t
}

// covers reports whether the range covers minute m of the week (0 = MON 00:00), spilling past midnight and from SUN
// into MON as painting does.
func (r *stateRange) covers(m int) bool {
	length := r.length()

	for _, d := range r.Days {
		if (m-(d*minutesPerDay+r.Start)+minutesPerWeek)%minutesPerWeek < length {
			return true
		}
	}

	return false
}

// length is the range's duration in minutes; start == end means 24 hours.
func (r *stateRange) length() int {
	if n := (r.End - r.Start + minutesPerDay) % minutesPerDay; n > 0 {
		return n
	}

	return minutesPerDay
}

// spans returns the per-day runs of the minutes of the week for which in reports true, and their total.
func spans(in func(m int) (int, bool)) ([]Span, int) {
	keyed, total := keyedSpans(in)

	out := make([]Span, 0, len(keyed))
	for _, k := range keyed {
		out = append(out, k.Span)
	}

	return out, total
}

// keyedSpans returns the per-day runs of equal key among the minutes of the week for which in reports true, and the
// number of those minutes.
func keyedSpans(in func(m int) (int, bool)) ([]keyedSpan, int) {
	var (
		out   []keyedSpan
		total int
	)

	for d := range expr.Weekdays {
		start, key := -1, 0

		for m := 0; m <= minutesPerDay; m++ {
			k, ok := 0, false
			if m < minutesPerDay {
				k, ok = in(d*minutesPerDay + m)
			}

			if start >= 0 && (!ok || k != key) {
				out = append(out, keyedSpan{Span: Span{
					Day: expr.Weekdays[d], Start: clock(start), End: clock(m % minutesPerDay),
				}, key: key})
				start = -1
			}

			if ok {
				total++

				if start < 0 {
					start, key = m, k
				}
			}
		}
	}

	return out, total
}

func dayNames(days []int) []string {
	out := make([]string, 0, len(days))
	for _, d := range days {
		out = append(out, expr.Weekdays[d])
	}

	return out
}

// blockLines is the line span of a directive block: from its marker to its last item.
func blockLines(d *document.Directive) Lines {
	l := Lines{From: d.KindPos.Line, To: d.KindPos.Line}
	if n := len(d.Items); n > 0 {
		l.To = max(l.To, d.Items[n-1].Pos.Line)
	}

	return l
}

// JSON encodes the trace and validates it against the embedded trace schema. A validation failure is a bug in this
// tool, not in the input.
func (t *Trace) JSON() ([]byte, error) {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")

	if err := enc.Encode(t); err != nil {
		return nil, fmt.Errorf("encode trace: %w", err)
	}

	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("decode trace: %w", err)
	}

	sch, err := traceSchema()
	if err != nil {
		return nil, err
	}

	if err := sch.Validate(doc); err != nil {
		return nil, fmt.Errorf("internal error, please report it: the trace fails its own schema: %w", err)
	}

	return buf.Bytes(), nil
}

// Lines returns one human-readable line per trace entry (base first, then ranges in source order), each with the
// source position it explains. The wording is not contract.
func (t *Trace) Lines() []TraceLine {
	out := make([]TraceLine, 0, len(t.Ranges)+1)

	base := fmt.Sprintf("%s is the base (%s): %s not painted over", t.Base.State, baseOrigin(t.Base.Origin),
		duration(t.Base.Minutes))
	out = append(out, TraceLine{Pos: diag.Pos{Line: t.Base.Line, Col: t.Base.Column}, Message: base})

	for i := range t.Ranges {
		r := &t.Ranges[i]
		msg := fmt.Sprintf("%s %s %s–%s: covers %s", r.State, compact(r.Days), r.Start, r.End, duration(r.Minutes))

		if len(r.Shadowed) == 0 {
			msg += ", all effective"
		} else {
			msg += fmt.Sprintf(", %s effective; shadowed: %s", duration(r.Minutes-shadowedMinutes(r.Shadowed)),
				shadowList(r.Shadowed))
		}

		out = append(out, TraceLine{Pos: diag.Pos{Line: r.Line, Col: r.Column}, Message: msg})
	}

	return out
}

func baseOrigin(origin string) string {
	switch origin {
	case BaseFromDocument:
		return "BASE block"
	case BaseFromFlag:
		return "--base"
	default:
		return "first configured state"
	}
}

func shadowedMinutes(s []ShadowedSpan) int {
	n := 0
	for _, x := range s {
		n += spanMinutes(x.Span)
	}

	return n
}

func spanMinutes(s Span) int {
	end := clockMinutes(s.End)
	if end == 0 {
		end = minutesPerDay
	}

	return end - clockMinutes(s.Start)
}

func clockMinutes(hhmm string) int {
	var h, m int

	_, _ = fmt.Sscanf(hhmm, "%d:%d", &h, &m)

	return h*minutesPerHour + m
}

func shadowList(s []ShadowedSpan) string {
	parts := make([]string, 0, len(s))
	for _, x := range s {
		parts = append(parts, fmt.Sprintf("%s %s–%s by line %d", x.Day, x.Start, x.End, x.By))
	}

	return strings.Join(parts, ", ")
}

// compact writes days as listed in the item, with runs of three or more consecutive days as ranges.
func compact(days []string) string {
	idx := make([]int, 0, len(days))
	for _, d := range days {
		idx = append(idx, slices.Index(expr.Weekdays, d))
	}

	var parts []string

	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && idx[j+1] == (idx[j]+1)%len(expr.Weekdays) {
			j++
		}

		switch {
		case j-i >= 2: //nolint:mnd // Three days or more read better as a range.
			parts = append(parts, expr.Weekdays[idx[i]]+"-"+expr.Weekdays[idx[j]])
		default:
			for k := i; k <= j; k++ {
				parts = append(parts, expr.Weekdays[idx[k]])
			}
		}

		i = j + 1
	}

	return strings.Join(parts, ",")
}

// duration renders minutes as "75 h", "2 h 30 min" or "45 min".
func duration(mins int) string {
	h, m := mins/minutesPerHour, mins%minutesPerHour

	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%d h %d min", h, m)
	case h > 0:
		return fmt.Sprintf("%d h", h)
	default:
		return fmt.Sprintf("%d min", m)
	}
}
