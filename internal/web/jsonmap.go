package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/iilei/canned-schedules/internal/render"
	"github.com/iilei/canned-schedules/internal/schedule"
)

const (
	minutesPerHour = 60
	minutesPerDay  = 24 * minutesPerHour
)

var (
	errNotJSON = errors.New("unexpected JSON token")
	dayNames   = []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}
)

type indexer struct {
	spans map[string]schedule.Lines
	dec   *json.Decoder
	text  []byte
}

// jsonSpans maps every value of the JSON text to its line span: "/name", "/annotations/x-owner", "/week/MON/2",
// "/transitions/0". A value's span starts at its key (or, in an array, at the value) and ends with the value.
func jsonSpans(text string) (map[string]schedule.Lines, error) {
	ix := indexer{text: []byte(text), dec: json.NewDecoder(strings.NewReader(text)), spans: map[string]schedule.Lines{}}
	if err := ix.value("", 1); err != nil {
		return nil, err
	}

	return ix.spans, nil
}

func (ix *indexer) line(off int64) int { return 1 + bytes.Count(ix.text[:off], []byte("\n")) }

// next returns the line of the next token, skipping whitespace and separators.
func (ix *indexer) next() int {
	off := ix.dec.InputOffset()
	for off < int64(len(ix.text)) && strings.IndexByte(" \t\r\n,:", ix.text[off]) >= 0 {
		off++
	}

	return ix.line(off)
}

// value reads one value at path, which starts on line from.
func (ix *indexer) value(path string, from int) error {
	tok, err := ix.dec.Token()
	if err != nil {
		return err
	}

	if d, ok := tok.(json.Delim); ok {
		if err := ix.children(path, d == '{'); err != nil {
			return err
		}
	}

	ix.spans[path] = schedule.Lines{From: from, To: ix.line(ix.dec.InputOffset())}

	return nil
}

// children reads the members of an object or the elements of an array, then the closing delimiter.
func (ix *indexer) children(path string, object bool) error {
	for i := 0; ix.dec.More(); i++ {
		child, line := path+"/"+strconv.Itoa(i), ix.next()

		if object {
			key, err := ix.dec.Token()
			if err != nil {
				return err
			}

			s, ok := key.(string)
			if !ok {
				return errNotJSON
			}

			child = path + "/" + s
		}

		if err := ix.value(child, line); err != nil {
			return err
		}
	}

	_, err := ix.dec.Token()

	return err
}

// attachJSON fills each highlight's JSON spans: the field a header entry produces, and for the base and the ranges
// the week runs and transitions where their state shows.
func attachJSON(hs []render.Highlight, out *schedule.Output, tr *schedule.Trace, text string) {
	spans, err := jsonSpans(text)
	if err != nil {
		return
	}

	for i := range hs {
		h := &hs[i]

		var paths []string

		switch h.Kind {
		case render.HighlightName:
			paths = []string{"/name"}
		case render.HighlightMetaBlock:
			paths = metaBlockPaths(tr)
		case render.HighlightMeta:
			paths = []string{"/annotations/" + h.Key}
			if h.Key == "schema-version" {
				paths = []string{"/schema-version"}
			}
		case render.HighlightBase, render.HighlightRange:
			paths = paintPaths(h.Owner, out, tr)
			if h.Kind == render.HighlightBase {
				paths = append(paths, "/base")
			}
		}

		h.JSON = merge(paths, spans)
	}
}

// metaBlockPaths covers everything the META block produces: the summary, the whole annotations object (braces
// included) when the block has x-* keys, and schema-version when it declares one.
func metaBlockPaths(tr *schedule.Trace) []string {
	paths := []string{"/summary"} // Absent from the JSON without a summary; merge skips it.

	if tr.Meta == nil {
		return paths
	}

	for _, e := range tr.Meta.Entries {
		if e.Key == "schema-version" {
			paths = append(paths, "/schema-version")
		} else {
			paths = append(paths, "/annotations")
		}
	}

	return paths
}

// paintPaths lists the week runs owner paints at least one minute of, and the transitions that switch to its state
// at a minute it paints.
func paintPaths(owner int, out *schedule.Output, tr *schedule.Trace) []string {
	var paths []string

	days := [][]schedule.Run{
		out.Week.MON, out.Week.TUE, out.Week.WED, out.Week.THU, out.Week.FRI, out.Week.SAT, out.Week.SUN,
	}

	for d, runs := range days {
		for i, r := range runs {
			end := clock(r.End)
			if end == 0 {
				end = minutesPerDay
			}

			for m := clock(r.Start); m < end; m++ {
				if tr.Owner(d*minutesPerDay+m) == owner {
					paths = append(paths, "/week/"+dayNames[d]+"/"+strconv.Itoa(i))

					break
				}
			}
		}
	}

	for i := range out.Transitions {
		if tr.Switches(owner, &out.Transitions[i]) {
			paths = append(paths, "/transitions/"+strconv.Itoa(i))
		}
	}

	return paths
}

// merge looks up paths and joins overlapping or adjacent spans, in line order.
func merge(paths []string, spans map[string]schedule.Lines) []schedule.Lines {
	var ls []schedule.Lines

	for _, p := range paths {
		if s, ok := spans[p]; ok {
			ls = append(ls, s)
		}
	}

	slices.SortFunc(ls, func(a, b schedule.Lines) int { return a.From - b.From })

	out := []schedule.Lines{}

	for _, l := range ls {
		if n := len(out); n > 0 && l.From <= out[n-1].To+1 {
			out[n-1].To = max(out[n-1].To, l.To)

			continue
		}

		out = append(out, l)
	}

	return out
}

func clock(hhmm string) int {
	h, m, _ := strings.Cut(hhmm, ":")
	hours, _ := strconv.Atoi(h)
	mins, _ := strconv.Atoi(m)

	return hours*minutesPerHour + mins
}
