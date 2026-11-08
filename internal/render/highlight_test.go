package render

import (
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

	return Highlights(tr, cfg, p), p
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
