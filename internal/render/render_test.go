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

	out, diags := schedule.Evaluate([]byte(src), cfg, schedule.Options{SampleInterval: 1})
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

		svg, err := SVG(out, cfg, 15, ThemeAuto)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}

		rects, err := countRects(svg)
		if err != nil {
			t.Errorf("%s: not well-formed XML: %v", in, err)

			continue
		}

		p := Downsample(out, cfg, 15)
		runs := 0

		for d := range days {
			runs += len(p.segments(d))
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

	svg, err := SVG(out, cfg, 15, ThemeAuto)
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

func TestBucketTicks(t *testing.T) {
	t.Parallel()

	out, cfg := evaluate(t, "# Plan\n\n> [!SCHEDULE:TIER] Medium\n> - `06:00` to `22:00` MON\n")

	for size, minor := range map[int]bool{60: false, 30: true, 15: true, 5: false, 1: false} {
		m := Build(out, cfg, Downsample(out, cfg, size), ThemeAuto)
		if got := len(m.Ticks) > hoursPerDay+1; got != minor {
			t.Errorf("bucket %d: minor ticks = %v, want %v", size, got, minor)
		}
	}
}

func TestDownsample(t *testing.T) {
	t.Parallel()

	// MON 06:07-09:52 Medium on base Off: 06:00 bucket has 8 Medium minutes, 09:45 bucket only 7.
	out, cfg := evaluate(
		t,
		"# Plan\n\n> [!SCHEDULE:BASE] Off\n\n> [!SCHEDULE:TIER] Medium\n> - `06:07` to `09:52` MON\n",
	)

	p := Downsample(out, cfg, 15)
	if p.Mixed != 2 {
		t.Errorf("mixed = %d, want 2", p.Mixed)
	}

	if got := p.segments(0); len(got) != 3 || got[1] != (segment{tier: "Medium", from: 360, to: 585}) {
		t.Errorf("MON segments = %+v", got)
	}

	if exact := Downsample(out, cfg, 1); exact.Mixed != 0 || len(exact.Days[0]) != minutesPerDay {
		t.Errorf("1-minute buckets must be exact: %+v", exact.Mixed)
	}

	// A 30/30 tie goes to the later tier in the config (Maximum after Off).
	out, cfg = evaluate(
		t,
		"# Plan\n\n> [!SCHEDULE:BASE] Off\n\n> [!SCHEDULE:TIER] Maximum\n> - `00:30` to `01:00` TUE\n",
	)
	if got := Downsample(out, cfg, 60).Days[1][0]; got != "Maximum" {
		t.Errorf("tie went to %s, want Maximum", got)
	}
}

func TestASCII(t *testing.T) {
	t.Parallel()

	out, cfg := evaluate(t, "# Plan\n\n> [!SCHEDULE:META] Office\n\n> [!SCHEDULE:BASE] Minimum\n\n"+
		"> [!SCHEDULE:TIER] Medium\n> - `06:00` to `22:00` MON-FRI\n")

	text, err := ASCII(out, cfg, 15, false)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(text), "\n")
	if lines[0] != "Plan - Office" || !strings.HasPrefix(lines[2], "    00:00") {
		t.Errorf("header = %q", lines[:3])
	}

	for _, l := range lines {
		if n := len([]rune(l)); n > 100 {
			t.Errorf("line has %d columns, want at most 100: %q", n, l)
		}
	}

	if want := "MON " + strings.Repeat(":", 24) + strings.Repeat("=", 64) + strings.Repeat(":", 8); lines[4] != want {
		t.Errorf("MON row = %q", lines[4])
	}

	if strings.Contains(string(text), "\x1b") {
		t.Error("no colour codes without color")
	}

	for _, r := range string(text) {
		if r > 127 {
			t.Fatalf("the default preview must be pure ASCII, found %q", r)
		}
	}

	colored, err := ASCII(out, cfg, 15, true)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(colored), "\x1b[48;2;") || !strings.Contains(string(colored), ansiReset) {
		t.Error("colour codes missing with color")
	}
}

func TestBlend(t *testing.T) {
	t.Parallel()

	fg, _ := parseHex("#FF000080") // Red at about half opacity.
	bg, _ := parseHex("#000000")

	if r, g, b := blend(fg, bg); r != 128 || g != 0 || b != 0 {
		t.Errorf("blend = %d,%d,%d, want 128,0,0", r, g, b)
	}

	if _, ok := parseHex("red"); ok {
		t.Error("named colours are not hex")
	}
}

func TestTooltipsAndTable(t *testing.T) {
	t.Parallel()

	out, cfg := evaluate(t, "# Plan\n\n> [!SCHEDULE:BASE] Minimum\n\n> [!SCHEDULE:TIER] Medium\n"+
		"> - `06:00` to `22:00` MON-FRI\n> - `22:00` to `02:00` SAT,SUN\n")
	m := Build(out, cfg, Downsample(out, cfg, 15), ThemeAuto)

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

func TestSVGThemes(t *testing.T) {
	t.Parallel()

	out, cfg := evaluate(t, "# Plan\n\n> [!SCHEDULE:TIER] Medium\n> - `06:00` to `22:00` MON\n")
	light, dark := cfg.Representation.Theme.Light.Background, cfg.Representation.Theme.Dark.Background

	for theme, want := range map[string]struct {
		has, lacks []string
	}{
		ThemeAuto:  {has: []string{light, dark, "prefers-color-scheme", "color-scheme: light dark"}},
		ThemeLight: {has: []string{light, "color-scheme: light"}, lacks: []string{dark, "prefers-color-scheme"}},
		ThemeDark:  {has: []string{dark, "color-scheme: dark"}, lacks: []string{light, "prefers-color-scheme"}},
	} {
		svg, err := SVG(out, cfg, 15, theme)
		if err != nil {
			t.Fatal(err)
		}

		for _, s := range want.has {
			if !strings.Contains(string(svg), s) {
				t.Errorf("%s: missing %q", theme, s)
			}
		}

		for _, s := range want.lacks {
			if strings.Contains(string(svg), s) {
				t.Errorf("%s: must not contain %q", theme, s)
			}
		}

		if strings.Contains(string(svg), "\n    .background") {
			t.Errorf("%s: rules must be scoped to .canned-schedule", theme)
		}
	}
}
