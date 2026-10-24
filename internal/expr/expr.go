// Package expr parses the range items of a TIER block: `HH:MM` to `HH:MM` WEEKDAYS.
package expr

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/document"
)

const (
	minutesPerHour = 60
	// itemTokens is the token count of a well-formed item: code, " to ", code, weekdays.
	itemTokens = 4
	// midnightAs24 gets its own message: it's the most likely mistake for "until midnight".
	midnightAs24 = "24:00"
)

// Weekdays lists the accepted weekday names, Monday first. The index is the day number used in Range.Days.
var Weekdays = []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}

var (
	clock     = regexp.MustCompile(`^([01]?\d|2[0-3]):([0-5]\d)$`)
	separator = regexp.MustCompile(`^[ \t]+to[ \t]+$`)
)

// Range is one parsed range item. Start and End are minutes since midnight; Start == End means 24 hours. Days are
// the start days (0 = MON), in the order written, wrap-around ranges expanded.
type Range struct {
	Days  []int
	Start int
	End   int
}

// dayToken is a word, "-" or "," in a weekday expression, with its position.
type dayToken struct {
	text string
	pos  diag.Pos
}

// Parse parses one range item. Every time must be a multiple of sampleInterval minutes.
func Parse(item document.Item, sampleInterval int) (Range, diag.List) {
	toks := item.Tokens
	if !wellFormed(toks) {
		return Range{}, diag.List{diag.Errorf(item.Pos, diag.CodeInvalidRange,
			"expected `HH:MM` to `HH:MM` WEEKDAYS, e.g. - `06:00` to `22:00` MON-FRI")}
	}

	var (
		r     Range
		diags diag.List
		ok    bool
	)

	if r.Start, ok = parseTime(toks[0], sampleInterval, &diags); !ok {
		return Range{}, diags
	}

	if r.End, ok = parseTime(toks[2], sampleInterval, &diags); !ok {
		return Range{}, diags
	}

	days, dayDiags := parseDays(toks[3])
	if len(dayDiags) > 0 {
		return Range{}, dayDiags
	}

	r.Days = days

	return r, nil
}

// wellFormed checks the token shape: code, " to ", code, whitespace plus weekdays, all on one line.
func wellFormed(toks []document.Token) bool {
	if len(toks) != itemTokens {
		return false
	}

	kinds := []document.TokenKind{document.TokenCode, document.TokenText, document.TokenCode, document.TokenText}
	for i, t := range toks {
		if t.Kind != kinds[i] || strings.Contains(t.Value, "\n") {
			return false
		}
	}

	days := toks[3].Value

	return separator.MatchString(toks[1].Value) &&
		strings.TrimLeft(days, " \t") != days && strings.TrimSpace(days) != ""
}

func parseTime(tok document.Token, sampleInterval int, diags *diag.List) (int, bool) {
	m := clock.FindStringSubmatch(tok.Value)
	if m == nil {
		*diags = append(*diags, invalidTime(tok))

		return 0, false
	}

	h, _ := strconv.Atoi(m[1])
	mins, _ := strconv.Atoi(m[2])
	total := h*minutesPerHour + mins

	if total%sampleInterval != 0 {
		*diags = append(*diags, diag.Errorf(tok.Pos, diag.CodeSampleInterval,
			"time %s is not a multiple of %d minutes (--sample-interval)", tok.Value, sampleInterval))

		return 0, false
	}

	return total, true
}

// parseDays parses `DAY[-DAY](,DAY[-DAY])*`. Whitespace anywhere in the expression is ignored.
func parseDays(tok document.Token) ([]int, diag.List) {
	toks := lexDays(tok)

	var days []int

	for i := 0; i < len(toks); {
		from, err := dayAt(toks, i, tok)
		if err != nil {
			return nil, diag.List{*err}
		}

		to := from
		i++

		if i < len(toks) && toks[i].text == "-" {
			if to, err = dayAt(toks, i+1, tok); err != nil {
				return nil, diag.List{*err}
			}

			i += 2
		}

		days = append(days, expand(from, to)...)

		if i < len(toks) {
			if toks[i].text != "," {
				return nil, diag.List{badDay(toks[i])}
			}

			i++

			if i == len(toks) {
				return nil, diag.List{badDay(toks[i-1])}
			}
		}
	}

	return days, nil
}

// invalidTime explains a malformed time. Examples stay on a 15-minute grid and avoid 23:59, which reads like a
// recommended end of day.
func invalidTime(tok document.Token) diag.Diagnostic {
	if tok.Value == midnightAs24 {
		return diag.Errorf(tok.Pos, diag.CodeInvalidTime,
			"24:00 is not a valid time: the end of a range is exclusive, so a range that runs until midnight ends at "+
				"00:00, e.g. `21:45` to `00:00`")
	}

	return diag.Errorf(tok.Pos, diag.CodeInvalidTime,
		"time %q: write H:MM or HH:MM with hours 0-23 and two-digit minutes, e.g. `6:15` or `21:45`", tok.Value)
}

// dayAt returns the weekday at toks[i], or an error diagnostic.
func dayAt(toks []dayToken, i int, src document.Token) (int, *diag.Diagnostic) {
	if i >= len(toks) {
		d := diag.Errorf(src.Pos, diag.CodeInvalidWeekday, "weekday expression %q is incomplete",
			strings.TrimSpace(src.Value))

		return 0, &d
	}

	for n, name := range Weekdays {
		if toks[i].text == name {
			return n, nil
		}
	}

	d := badDay(toks[i])

	return 0, &d
}

func badDay(t dayToken) diag.Diagnostic {
	return diag.Errorf(t.pos, diag.CodeInvalidWeekday,
		"%q: use MON TUE WED THU FRI SAT SUN (uppercase), ranges like MON-FRI and lists like SAT,SUN", t.text)
}

// expand returns the days from..to inclusive, wrapping around the week (FRI-MON = FRI SAT SUN MON).
func expand(from, to int) []int {
	days := []int{from}
	for d := from; d != to; {
		d = (d + 1) % len(Weekdays)
		days = append(days, d)
	}

	return days
}

func lexDays(tok document.Token) []dayToken {
	var (
		out  []dayToken
		word strings.Builder
		at   diag.Pos
	)

	flush := func() {
		if word.Len() > 0 {
			out = append(out, dayToken{text: word.String(), pos: at})
			word.Reset()
		}
	}

	col := tok.Pos.Col
	for _, r := range tok.Value {
		pos := diag.Pos{Line: tok.Pos.Line, Col: col}
		col++ // Columns count runes.

		switch r {
		case ' ', '\t':
			flush()
		case '-', ',':
			flush()
			out = append(out, dayToken{text: string(r), pos: pos})
		default:
			if word.Len() == 0 {
				at = pos
			}

			word.WriteRune(r)
		}
	}

	flush()

	return out
}
