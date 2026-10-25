package document

import (
	"testing"

	"github.com/iilei/canned-schedules/internal/diag"
)

func TestParseGoldenFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "syntax shown in code spans is not a misplaced directive",
			src:  "# Plan\n\nWrite `> [!SCHEDULE:TIER] Medium` followed by ranges.\n",
			want: []string{},
		},
		{
			name: "directive-like text in prose is misplaced",
			src:  "# Plan\n\nSee [!SCHEDULE:TIER] below.\n",
			want: []string{"error 3:7 misplaced-directive"},
		},
		{
			name: "ordered list in a tier",
			src:  "# Plan\n\n> [!SCHEDULE:TIER] Medium\n> 1. `06:00` to `22:00` MON\n",
			want: []string{"error 4:6 invalid-range"},
		},
		{
			name: "content in a base block",
			src:  "# Plan\n\n> [!SCHEDULE:BASE] Minimum\n> - `06:00` to `22:00` MON\n",
			want: []string{"error 4:5 unexpected-content"},
		},
		{
			name: "malformed meta line",
			src:  "# Plan\n\n> [!SCHEDULE:META]\n> owner team\n",
			want: []string{"error 4:3 meta-schema"},
		},
		{
			name: "columns count runes, not bytes",
			src:  "# Plan\n\n> [!SCHEDULE:TIER] Medium\n> - `06:00` to `22:00` MON\n\nÄÖÜ [!SCHEDULE:BASE]\n",
			want: []string{"error 6:7 misplaced-directive"},
		},
		{
			name: "diagnostics are sorted by position",
			src:  "> [!SCHEDULE:NOPE] x\n\n# 1st\n",
			want: []string{"error 1:5 unknown-kind", "error 3:3 invalid-name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, diags := Parse([]byte(tt.src))
			if got := golden(diags); !equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseDocument(t *testing.T) {
	t.Parallel()

	src := "# Office Hours\n\n> [!schedule:meta] Weekdays only\n> - x-owner: `ops`\n\n" +
		"> [!SCHEDULE:BASE] Minimum\n\n> [!SCHEDULE:TIER] Medium\n> - `06:00` to `22:00` MON-FRI\n"

	doc, diags := Parse([]byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %q", golden(diags))
	}

	if doc.Name.Value != "Office Hours" || doc.Name.Pos != (diag.Pos{Line: 1, Col: 3}) {
		t.Errorf("name = %+v", doc.Name)
	}

	if doc.Meta == nil || doc.Meta.Title.Value != "Weekdays only" {
		t.Fatalf("meta = %+v", doc.Meta)
	}

	if doc.Base == nil || doc.Base.Title.Value != "Minimum" {
		t.Fatalf("base = %+v", doc.Base)
	}

	if len(doc.Tiers) != 1 || len(doc.Tiers[0].Items) != 1 {
		t.Fatalf("tiers = %+v", doc.Tiers)
	}

	toks := doc.Tiers[0].Items[0].Tokens
	want := []Token{
		{Kind: TokenCode, Value: "06:00", Pos: diag.Pos{Line: 9, Col: 6}},
		{Kind: TokenText, Value: " to ", Pos: diag.Pos{Line: 9, Col: 12}},
		{Kind: TokenCode, Value: "22:00", Pos: diag.Pos{Line: 9, Col: 17}},
		{Kind: TokenText, Value: " MON-FRI", Pos: diag.Pos{Line: 9, Col: 23}},
	}

	if !equalTokens(toks, want) {
		t.Errorf("tokens = %+v, want %+v", toks, want)
	}
}

func golden(diags diag.List) []string {
	out := []string{}
	for _, d := range diags {
		out = append(out, d.Golden())
	}

	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func equalTokens(a, b []Token) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
