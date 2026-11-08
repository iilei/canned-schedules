package web

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/iilei/canned-schedules/internal/config"
)

// minInkContrast is the WCAG AA contrast for normal text, which the better of the two highlight inks must reach
// against every state fill: thin stripes and outlines need at least that.
const minInkContrast = 4.5

type rgba struct{ r, g, b, a float64 }

// TestHighlightInkContrast checks the promise behind the page's black-and-white highlight: on every state fill of
// every palette (the config's own included), in both themes, as drawn (blended over the theme background), black or
// white contrasts at least 4.5:1. The inks are read from app.js, so changing them there is covered too.
func TestHighlightInkContrast(t *testing.T) {
	t.Parallel()

	app, err := os.ReadFile("../../web/app.js")
	if err != nil {
		t.Fatal(err)
	}

	m := regexp.MustCompile(`const INK = \{ dark: "(#[0-9a-fA-F]{6})", light: "(#[0-9a-fA-F]{6})" \};`).
		FindSubmatch(app)
	if m == nil {
		t.Fatal("app.js: no INK constant")
	}

	dark, light := hexColor(t, string(m[1])), hexColor(t, string(m[2]))

	cfg, diags := config.Load(nil)
	if cfg == nil {
		t.Fatal(diags)
	}

	sets := map[string][]config.Style{builtIn: cfg.Representation.Styles.States}

	raw, err := os.ReadFile(paletteIndex)
	if err != nil {
		t.Fatal(err)
	}

	var list []palette
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}

	for _, p := range list {
		if len(p.Styles) == 0 || string(p.Styles) == "null" {
			continue
		}

		var styles []config.Style
		if err := json.Unmarshal(p.Styles, &styles); err != nil {
			t.Fatalf("%s: %v", p.ID, err)
		}

		sets[p.ID] = styles
	}

	theme := cfg.Representation.Theme

	for id, styles := range sets {
		for i, s := range styles {
			for _, c := range []struct{ scheme, fill, background string }{
				{"light", s.Color.Light, theme.Light.Background},
				{"dark", s.Color.Dark, theme.Dark.Background},
			} {
				fill := over(hexColor(t, c.fill), hexColor(t, c.background))
				if best := math.Max(contrast(dark, fill), contrast(light, fill)); best < minInkContrast {
					t.Errorf("%s style %d (%s, %s): best ink contrast %.2f, want at least %.1f", id, i, c.scheme,
						c.fill, best, minInkContrast)
				}
			}
		}
	}
}

func hexColor(t *testing.T, s string) rgba {
	t.Helper()

	if len(s) != 7 && len(s) != 9 {
		t.Fatalf("colour %q: want #RRGGBB or #RRGGBBAA", s)
	}

	channel := func(i int) float64 {
		v, err := strconv.ParseUint(s[i:i+2], 16, 8)
		if err != nil {
			t.Fatalf("colour %q: %v", s, err)
		}

		return float64(v) / 255
	}

	c := rgba{r: channel(1), g: channel(3), b: channel(5), a: 1}
	if len(s) == 9 {
		c.a = channel(7)
	}

	return c
}

// over blends fg over an opaque bg, as the browser draws a translucent state fill.
func over(fg, bg rgba) rgba {
	mix := func(f, b float64) float64 { return f*fg.a + b*(1-fg.a) }

	return rgba{r: mix(fg.r, bg.r), g: mix(fg.g, bg.g), b: mix(fg.b, bg.b), a: 1}
}

// contrast is the WCAG 2 contrast ratio of two opaque colours.
func contrast(x, y rgba) float64 {
	lx, ly := luminance(x), luminance(y)

	return (math.Max(lx, ly) + 0.05) / (math.Min(lx, ly) + 0.05)
}

func luminance(c rgba) float64 {
	lin := func(v float64) float64 {
		if v <= 0.03928 {
			return v / 12.92
		}

		return math.Pow((v+0.055)/1.055, 2.4)
	}

	return 0.2126*lin(c.r) + 0.7152*lin(c.g) + 0.0722*lin(c.b)
}
