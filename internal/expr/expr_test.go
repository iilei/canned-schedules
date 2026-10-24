package expr

import (
	"reflect"
	"strings"
	"testing"

	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/document"
)

// item builds a well-formed item; positions start at line 1, column 1 for the first code span's content.
func item(start, sep, end, days string) document.Item {
	return document.Item{
		Pos: diag.Pos{Line: 1, Col: 1},
		Tokens: []document.Token{
			{Kind: document.TokenCode, Value: start, Pos: diag.Pos{Line: 1, Col: 2}},
			{Kind: document.TokenText, Value: sep, Pos: diag.Pos{Line: 1, Col: 20}},
			{Kind: document.TokenCode, Value: end, Pos: diag.Pos{Line: 1, Col: 30}},
			{Kind: document.TokenText, Value: days, Pos: diag.Pos{Line: 1, Col: 40}},
		},
	}
}

func TestParseValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item document.Item
		want Range
	}{
		{"single day", item("06:00", " to ", "22:00", " MON"), Range{Start: 360, End: 1320, Days: []int{0}}},
		{"single-digit hour", item("6:05", " to ", "9:30", " TUE"), Range{Start: 365, End: 570, Days: []int{1}}},
		{"range", item("00:00", " to ", "00:00", " MON-WED"), Range{Start: 0, End: 0, Days: []int{0, 1, 2}}},
		{
			"wrap-around",
			item("09:00", " to ", "17:00", " SAT-TUE"),
			Range{Start: 540, End: 1020, Days: []int{5, 6, 0, 1}},
		},
		{
			"list and range",
			item("09:00", " to ", "17:00", " MON-TUE,FRI"),
			Range{Start: 540, End: 1020, Days: []int{0, 1, 4}},
		},
		{
			"whitespace everywhere",
			item("09:00", "\tto  ", "17:00", "  MON - TUE ,  FRI  "),
			Range{Start: 540, End: 1020, Days: []int{0, 1, 4}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, diags := Parse(tt.item, 1)
			if len(diags) != 0 {
				t.Fatalf("unexpected diagnostics: %+v", diags)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		item           document.Item
		sampleInterval int
		want           string
	}{
		{"no space before weekdays", item("06:00", " to ", "22:00", "MON"), 1, "error 1:1 invalid-range"},
		{"uppercase TO", item("06:00", " TO ", "22:00", " MON"), 1, "error 1:1 invalid-range"},
		{"no spaces around to", item("06:00", "to", "22:00", " MON"), 1, "error 1:1 invalid-range"},
		{"weekdays only whitespace", item("06:00", " to ", "22:00", "   "), 1, "error 1:1 invalid-range"},
		{"two lines", item("06:00", " to\n", "22:00", " MON"), 1, "error 1:1 invalid-range"},
		{"end time invalid", item("06:00", " to ", "25:00", " MON"), 1, "error 1:30 invalid-time"},
		{"end time off the sample interval", item("06:00", " to ", "22:30", " MON"), 60, "error 1:30 sample-interval"},
		{"trailing range dash", item("06:00", " to ", "22:00", " MON-"), 1, "error 1:40 invalid-weekday"},
		{"leading comma", item("06:00", " to ", "22:00", " ,MON"), 1, "error 1:41 invalid-weekday"},
		{"trailing comma", item("06:00", " to ", "22:00", " MON,"), 1, "error 1:44 invalid-weekday"},
		{"double comma", item("06:00", " to ", "22:00", " MON,,TUE"), 1, "error 1:45 invalid-weekday"},
		{"chained range", item("06:00", " to ", "22:00", " MON-TUE-WED"), 1, "error 1:48 invalid-weekday"},
		{"missing comma", item("06:00", " to ", "22:00", " MON TUE"), 1, "error 1:45 invalid-weekday"},
		{"bad second day", item("06:00", " to ", "22:00", " MON-fri"), 1, "error 1:45 invalid-weekday"},
		{"one-day range", item("06:00", " to ", "22:00", " WED-WED"), 1, "error 1:45 duplicate-weekday"},
		{"day listed twice", item("06:00", " to ", "22:00", " MON,TUE,MON"), 1, "error 1:49 duplicate-weekday"},
		{"range end listed again", item("06:00", " to ", "22:00", " MON-WED,WED"), 1, "error 1:49 duplicate-weekday"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, diags := Parse(tt.item, tt.sampleInterval)
			if len(diags) != 1 || diags[0].Golden() != tt.want {
				t.Errorf("got %+v, want %q", diags, tt.want)
			}
		})
	}
}

func TestInvalidTimeMessages(t *testing.T) {
	t.Parallel()

	_, diags := Parse(item("10:00", " to ", "24:00", " SAT"), 1)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "exclusive") ||
		!strings.Contains(diags[0].Message, "00:00") {
		t.Errorf("24:00 must explain the exclusive end and point to 00:00, got %+v", diags)
	}

	if diags[0].Example != "`21:45` to `00:00`" || strings.Contains(diags[0].Message, "e.g.") {
		t.Errorf("the example belongs in Example, not in the message: %+v", diags[0])
	}

	_, diags = Parse(item("noon", " to ", "14:00", " SAT"), 1)
	if len(diags) != 1 || strings.Contains(diags[0].Message, "23:59") {
		t.Errorf("examples must not suggest 23:59, got %+v", diags)
	}
}

func TestRepeatedWeekdayWarns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		days string
		want string
		span []int
	}{
		{days: " MON-WED,TUE", want: "warn 1:49 repeated-weekday", span: []int{0, 1, 2, 1}},
		{days: " FRI-MON,SUN", want: "warn 1:49 repeated-weekday", span: []int{4, 5, 6, 0, 6}},
		{days: " MON-THU,TUE,WED", want: "warn 1:49 repeated-weekday", span: []int{0, 1, 2, 3, 1, 2}},
	}

	for _, tt := range tests {
		r, diags := Parse(item("09:00", " to ", "17:00", tt.days), 1)
		if len(diags) != 1 || diags[0].Golden() != tt.want {
			t.Errorf("%s: got %+v, want one %q", tt.days, diags, tt.want)
		}

		if !reflect.DeepEqual(r.Days, tt.span) {
			t.Errorf("%s: a warning must keep the range: days = %v, want %v", tt.days, r.Days, tt.span)
		}
	}
}
