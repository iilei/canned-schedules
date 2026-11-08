package render

import (
	"strings"
	"testing"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/schedule"
)

const overlap = "# Plan\n\n> [!SCHEDULE-BASE] Minimum\n\n" +
	"> [!SCHEDULE-STATE] Medium\n> - `06:10` to `22:00` MON-FRI\n\n" +
	"> [!SCHEDULE-STATE] Maximum\n> - `20:00` to `02:00` FRI\n"

func highlights(t *testing.T, src string, size int) ([]Highlight, Preview) {
	t.Helper()

	out, cfg := evaluate(t, src)

	tr, diags := schedule.Explain([]byte(src), cfg, schedule.Options{SampleInterval: 1})
	if tr == nil {
		t.Fatalf("explain: %v", diags)
	}

	p := Downsample(out, cfg, size)
	m := Build(out, cfg, p, SVGOptions{Theme: ThemeAuto, Annotations: true})

	all := Highlights(tr, cfg, p, &m)

	// The paint highlights (base, ranges) follow the header ones.
	for i, h := range all {
		if h.Kind == HighlightBase {
			return all[i:], p
		}
	}

	t.Fatal("no base highlight")

	return nil, p
}

func TestHighlightsExactAtBucketOne(t *testing.T) {
	t.Parallel()

	hs, _ := highlights(t, overlap, 1)
	if len(hs) != 3 || hs[0].Kind != HighlightBase || hs[0].Line != 3 || hs[1].Line != 6 || hs[2].Line != 9 {
		t.Fatalf("highlights = %+v", hs)
	}

	week := float64(titleH + rulerH)
	fri := week + 4*dayH

	// Maximum spills from FRI into SAT: one rect per day, exact minutes.
	want := []Rect{
		{X: labelW + px(20*60), Y: fri, Width: px(4 * 60), Height: blockH},
		{X: labelW, Y: fri + dayH, Width: px(2 * 60), Height: blockH},
	}
	if got := hs[2].Rects; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("maximum rects = %+v, want %+v", got, want)
	}

	// Medium is shadowed on FRI 20:00–22:00.
	wantShadow := Rect{X: labelW + px(20*60), Y: fri, Width: px(2 * 60), Height: blockH}
	if got := hs[1].Shadowed; len(got) != 1 || got[0] != wantShadow {
		t.Errorf("medium shadowed = %+v, want %+v", got, wantShadow)
	}

	if hs[1].Block != (schedule.Lines{From: 5, To: 6}) || hs[0].Block != (schedule.Lines{From: 3, To: 3}) {
		t.Errorf("blocks = %+v, %+v", hs[1].Block, hs[0].Block)
	}
}

func TestHighlightsFollowBuckets(t *testing.T) {
	t.Parallel()

	hs, p := highlights(t, overlap, 15)
	medium := hs[1]

	// 06:10 covers 5 of the 06:00 bucket's 15 minutes: the bucket shows the base, so Medium starts at 06:15.
	if got := medium.Rects[0]; got.X != labelW+px(6*60+15) {
		t.Errorf("first medium rect starts at x %v, want the 06:15 bucket", got.X)
	}

	if len(medium.Shadowed) != 1 || medium.Label == "" {
		t.Errorf("medium = %+v", medium)
	}

	// Every rect of an entry sits on buckets that show the entry's state.
	for _, h := range hs {
		for _, r := range h.Rects {
			d := int((r.Y - float64(titleH+rulerH)) / dayH)
			for x := r.X; x < r.X+r.Width-0.01; x += px(p.Size) {
				b := int((x - labelW) / px(p.Size))
				if got := p.Days[d][b]; got != h.State {
					t.Fatalf("%s line %d: bucket %d of day %d shows %s", h.State, h.Line, b, d, got)
				}
			}
		}
	}
}

func TestHighlightsBaseWithoutBlock(t *testing.T) {
	t.Parallel()

	hs, _ := highlights(t, "# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON\n", 15)

	cfg, _ := config.Load(nil)
	if hs[0].Line != 0 || hs[0].Block.From != 0 || hs[0].State != cfg.States[0].ID ||
		len(hs[0].Rects) != 8 { // MON before and after the range, then six full days.
		t.Errorf("base = %+v", hs[0])
	}
}

func TestAnnotationsSectionIsOptIn(t *testing.T) {
	t.Parallel()

	src := "# Plan\n\n> [!SCHEDULE-META] Late Fridays\n> - x-zeta: `z`\n> - x-alpha: `a`\n\n" +
		"> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON\n"
	out, cfg := evaluate(t, src)

	plain, err := SVG(out, cfg, 15, SVGOptions{Theme: ThemeAuto})
	if err != nil || strings.Contains(string(plain), "annotations") || strings.Contains(string(plain), "x-alpha") {
		t.Fatalf("annotations must be opt-in: %v", err)
	}

	with, err := SVG(out, cfg, 15, SVGOptions{Theme: ThemeAuto, Annotations: true})
	if err != nil {
		t.Fatal(err)
	}

	s := string(with)
	if a, z := strings.Index(s, ">x-alpha<"), strings.Index(s, ">x-zeta<"); a < 0 || z < a {
		t.Errorf("rows must be sorted by key like the JSON:\n%s", s)
	}

	tr, _ := schedule.Explain([]byte(src), cfg, schedule.Options{SampleInterval: 1})
	p := Downsample(out, cfg, 15)
	m := Build(out, cfg, p, SVGOptions{Theme: ThemeAuto, Annotations: true})

	hs := Highlights(tr, cfg, p, &m)
	// The META block entry frames the summary and the whole section.
	if hs[0].Kind != HighlightName || len(hs[0].Rects) != 1 || hs[1].Kind != HighlightMetaBlock ||
		hs[1].Line != 3 || hs[1].Block != (schedule.Lines{From: 3, To: 5}) || len(hs[1].Rects) != 2 ||
		hs[2].Key != "x-zeta" || len(hs[2].Rects) != 1 {
		t.Errorf("header highlights = %+v", hs[:4])
	}

	if sec, row := hs[1].Rects[1], hs[2].Rects[0]; sec.Y > row.Y || sec.Y+sec.Height < row.Y+row.Height {
		t.Errorf("the section frame %+v must enclose the row frame %+v", sec, row)
	}

	// The frame of x-zeta sits on its row, below x-alpha's.
	if hs[2].Rects[0].Y <= hs[3].Rects[0].Y {
		t.Errorf("x-zeta frame at y %v, x-alpha at %v", hs[2].Rects[0].Y, hs[3].Rects[0].Y)
	}
}
