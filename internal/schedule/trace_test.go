package schedule

import (
	"reflect"
	"strings"
	"testing"

	"github.com/iilei/canned-schedules/internal/config"
)

func explain(t *testing.T, src string, opts Options) *Trace {
	t.Helper()

	cfg, diags := config.Load(nil)
	if cfg == nil {
		t.Fatal(diags)
	}

	if opts.SampleInterval == 0 {
		opts.SampleInterval = 1
	}

	tr, diags := Explain([]byte(src), cfg, opts)
	if tr == nil {
		t.Fatalf("Explain: %v", diags)
	}

	if _, err := tr.JSON(); err != nil {
		t.Fatalf("trace fails its schema: %v", err)
	}

	return tr
}

func TestTraceOverlap(t *testing.T) {
	t.Parallel()

	tr := explain(t, `# Plan

> [!SCHEDULE-BASE] Minimum

> [!SCHEDULE-STATE] Medium
> - `+"`06:00` to `22:00`"+` MON-FRI

> [!SCHEDULE-STATE] Maximum
> - `+"`20:00` to `02:00`"+` FRI
`, Options{})

	if tr.Base.Origin != BaseFromDocument || tr.Base.Line != 3 {
		t.Errorf("base = %+v", tr.Base)
	}

	med, maxi := tr.Ranges[0], tr.Ranges[1]

	if med.Line != 6 || med.Block != (Lines{From: 5, To: 6}) || med.Minutes != 5*16*60 {
		t.Errorf("medium = %+v", med)
	}

	wantShadow := []ShadowedSpan{{Span: Span{Day: "FRI", Start: "20:00", End: "22:00"}, By: 9}}
	if !reflect.DeepEqual(med.Shadowed, wantShadow) {
		t.Errorf("medium shadowed = %+v, want %+v", med.Shadowed, wantShadow)
	}

	// Spills into SAT; the FRI part ends at the end of the day.
	wantCovers := []Span{{Day: "FRI", Start: "20:00", End: "00:00"}, {Day: "SAT", Start: "00:00", End: "02:00"}}
	if !reflect.DeepEqual(maxi.Covers, wantCovers) || !reflect.DeepEqual(maxi.Effective, wantCovers) ||
		len(maxi.Shadowed) != 0 {
		t.Errorf("maximum = %+v", maxi)
	}

	if want := 7*24*60 - med.Minutes - maxi.Minutes + 2*60; tr.Base.Minutes != want {
		t.Errorf("base minutes = %d, want %d", tr.Base.Minutes, want)
	}

	lines := tr.Lines()
	if len(lines) != 3 || lines[1].Pos.Line != 6 || !strings.Contains(lines[1].Message, "FRI 20:00–22:00 by line 9") {
		t.Errorf("lines = %+v", lines)
	}
}

func TestTraceWrapAndFullDay(t *testing.T) {
	t.Parallel()

	tr := explain(t, "# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `22:00` to `01:00` SUN\n> - `08:00` to `08:00` WED\n",
		Options{Base: "Off"})

	if tr.Base.Origin != BaseFromFlag || tr.Base.Line != 0 {
		t.Errorf("base = %+v", tr.Base)
	}

	wantWrap := []Span{{Day: "MON", Start: "00:00", End: "01:00"}, {Day: "SUN", Start: "22:00", End: "00:00"}}
	if got := tr.Ranges[0].Covers; !reflect.DeepEqual(got, wantWrap) {
		t.Errorf("SUN→MON covers = %+v, want %+v", got, wantWrap)
	}

	if got := tr.Ranges[1].Minutes; got != 24*60 {
		t.Errorf("08:00 to 08:00 covers %d minutes, want 1440", got)
	}

	for m := range minutesPerWeek {
		if tr.Owner(m) >= 0 != (tr.Covers(0, m) || tr.Covers(1, m)) {
			t.Fatalf("minute %d: owner %d disagrees with coverage", m, tr.Owner(m))
		}
	}
}

func TestTraceBaseFromConfig(t *testing.T) {
	t.Parallel()

	tr := explain(t, "# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON\n", Options{})
	if tr.Base.Origin != BaseFromConfig || tr.State(-1) != tr.Base.State || tr.State(0) != "Medium" {
		t.Errorf("base = %+v", tr.Base)
	}
}

func TestTraceSummary(t *testing.T) {
	t.Parallel()

	tr := explain(t, "# Plan\n\n> [!SCHEDULE-BASE] Off\n\n"+
		"> [!SCHEDULE-STATE] Minimum\n> - `07:00` to `20:00` MON-FRI\n\n"+
		"> [!SCHEDULE-STATE] Maximum\n> - `12:00` to `13:00` MON-FRI\n> - `11:00` to `14:00` MON-FRI\n", Options{})

	for owner, want := range map[int]string{
		-1: "Off is the base (BASE block) · 103 h not painted over",
		0:  "Minimum MON-FRI 07:00–20:00 · 50 h of 65 h effective",
		1:  "Maximum MON-FRI 12:00–13:00 · no effect",
		2:  "Maximum MON-FRI 11:00–14:00 · all 15 h effective",
	} {
		if got := tr.Summary(owner); got != want {
			t.Errorf("Summary(%d) = %q, want %q", owner, got, want)
		}
	}
}
