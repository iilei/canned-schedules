package render

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/schedule"
)

func evaluate(t *testing.T, src string, overrides ...config.Override) (*schedule.Output, *config.Config) {
	t.Helper()

	cfg, diags := config.Load(overrides)
	if cfg == nil {
		t.Fatalf("config: %v", diags)
	}

	out, diags := schedule.Evaluate([]byte(src), cfg, schedule.Options{Granularity: 1})
	if out == nil {
		t.Fatalf("evaluate: %v", diags)
	}

	return out, cfg
}

// TestGoldenSchedulesRender renders every valid golden case: well-formed XML, one row per day, one block per run.
func TestGoldenSchedulesRender(t *testing.T) {
	t.Parallel()

	inputs, err := filepath.Glob("../../testdata/golden/valid/*/input.md")
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}

	for _, in := range inputs {
		src, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}

		out, cfg := evaluate(t, string(src))

		svg, err := SVG(out, cfg)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}

		rects, err := countRects(svg)
		if err != nil {
			t.Errorf("%s: not well-formed XML: %v", in, err)

			continue
		}

		runs := 0
		for d := range days {
			runs += len(dayRuns(out, d))
		}

		// Background + runs + one legend swatch per tier.
		if want := 1 + runs + len(cfg.Tiers); rects != want {
			t.Errorf("%s: %d rects, want %d", in, rects, want)
		}
	}
}

func countRects(svg []byte) (int, error) {
	dec := xml.NewDecoder(bytes.NewReader(svg))
	n := 0

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return n, nil
		}

		if err != nil {
			return n, err
		}

		if el, ok := tok.(xml.StartElement); ok && el.Name.Local == "rect" {
			n++
		}
	}
}

func TestEscaping(t *testing.T) {
	t.Parallel()

	out, cfg := evaluate(t, "# Plan\n\n> [!SCHEDULE:TIER] Maximum\n> - `06:00` to `22:00` MON\n",
		config.Override{Path: "tiers[3].display-name", Value: `"Max <b>&amp;</b> \"q\" \u0007"`})

	svg, err := SVG(out, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := countRects(svg); err != nil {
		t.Fatalf("hostile display name broke the XML: %v", err)
	}

	if !strings.Contains(string(svg), "Max &lt;b&gt;&amp;amp;&lt;/b&gt; &#34;q&#34; \\u0007") {
		t.Errorf("display name not escaped and sanitized:\n%s", svg)
	}
}

func TestResolutionAndTicks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src        string
		resolution int
		minorTicks bool
	}{
		{src: "`06:00` to `22:00` MON", resolution: 60},
		{src: "`06:30` to `22:00` MON", resolution: 30, minorTicks: true},
		{src: "`06:15` to `21:45` MON", resolution: 15, minorTicks: true},
		{src: "`06:05` to `22:00` MON", resolution: 5},
	}

	for _, tt := range tests {
		out, cfg := evaluate(t, "# Plan\n\n> [!SCHEDULE:TIER] Medium\n> - "+tt.src+"\n")
		m := Build(out, cfg)

		if m.Resolution != tt.resolution {
			t.Errorf("%s: resolution %d, want %d", tt.src, m.Resolution, tt.resolution)
		}

		hourTicks := hoursPerDay + 1
		if got := len(m.Ticks) > hourTicks; got != tt.minorTicks {
			t.Errorf("%s: minor ticks = %v, want %v (%d ticks)", tt.src, got, tt.minorTicks, len(m.Ticks))
		}
	}
}

func TestTooltipsAndTable(t *testing.T) {
	t.Parallel()

	out, cfg := evaluate(t, "# Plan\n\n> [!SCHEDULE:BASE] Minimum\n\n> [!SCHEDULE:TIER] Medium\n"+
		"> - `06:00` to `22:00` MON-FRI\n> - `22:00` to `02:00` SAT,SUN\n")
	m := Build(out, cfg)

	mon := m.Days[0].Runs
	if len(mon) != 4 || !strings.Contains(mon[2].Tooltip, "Medium · MON 06:00–22:00\nstarts: posix 0 6 * * 1-5") {
		t.Errorf("MON runs = %+v", mon)
	}

	if mon[0].Tooltip != "Medium · MON 00:00–02:00" {
		t.Errorf("the spill from SUN into MON has no transition of its own: %q", mon[0].Tooltip)
	}

	if !strings.Contains(mon[1].Tooltip, "Minimum · MON 02:00–06:00\nstarts: posix 0 2 * * 0-1") {
		t.Errorf("MON 02:00 run = %q", mon[1].Tooltip)
	}

	// SAT and SUN ranges spill into SUN and MON, so the switch back to the base happens on both days.
	if got := m.Transitions.Rows[0][0].Text; got != "MON,SUN 02:00 → Minimum" {
		t.Errorf("first row = %q", got)
	}

	if m.Legend.Swatches[1].Text.Text != "Minimum (base)" {
		t.Errorf("legend = %+v", m.Legend.Swatches)
	}
}

func TestCompactDays(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"MON TUE WED THU FRI":         "MON-FRI",
		"MON SAT SUN":                 "MON,SAT-SUN",
		"MON WED FRI":                 "MON,WED,FRI",
		"MON TUE WED THU FRI SAT SUN": "MON-SUN",
	} {
		if got := compactDays(strings.Fields(in)); got != want {
			t.Errorf("compactDays(%s) = %q, want %q", in, got, want)
		}
	}
}
