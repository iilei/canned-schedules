package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/schedule"
	"github.com/iilei/canned-schedules/templates"
)

const (
	asciiLabelW = 4 // "MON " in front of every row.
	labelLen    = len("00:00")
	ansiReset   = "\x1b[0m"
	hexRGB      = len("#RRGGBB")
	hexRGBA     = len("#RRGGBBAA")
	maxChannel  = 255
	halfHour    = 30
)

// ASCIIModel is the data the ASCII template operates on. Rows and the legend may contain ANSI colour codes.
type ASCIIModel struct {
	Title  string     `json:"title"`
	Legend string     `json:"legend"`
	Note   string     `json:"note"`
	Ruler  []string   `json:"ruler"`
	Rows   []ASCIIRow `json:"rows"`
}

// ASCIIRow is one day.
type ASCIIRow struct {
	Label string `json:"label"`
	Cells string `json:"cells"`
}

// rgba is a colour with alpha in 0..255.
type rgba [4]int

// ASCII renders out as text with buckets of size minutes: one row per day, one symbol per bucket (96 columns at 15
// minutes). With color, symbols get 24-bit ANSI colours from the dark theme.
func ASCII(out *schedule.Output, cfg *config.Config, size int, color bool) ([]byte, error) {
	return execute(templates.ScheduleASCII, BuildASCII(out, cfg, Downsample(out, cfg, size), color))
}

// BuildASCII computes the ASCII model for the downsampled week p.
func BuildASCII(out *schedule.Output, cfg *config.Config, p Preview, color bool) ASCIIModel {
	paint := painter(cfg, color)
	m := ASCIIModel{Title: diag.Sanitize(out.Name), Ruler: asciiRuler(p.Size)}

	if out.Summary != "" {
		m.Title += " - " + diag.Sanitize(out.Summary)
	}

	for d, name := range days {
		var cells strings.Builder

		for _, sg := range p.segments(d) {
			i, _ := stateInfo(cfg, sg.state)
			cells.WriteString(paint(i, strings.Repeat(cfg.Symbol(i), (sg.to-sg.from)/p.Size)))
		}

		m.Rows = append(m.Rows, ASCIIRow{Label: name, Cells: cells.String()})
	}

	m.Legend = asciiLegend(out, cfg, paint, asciiLabelW+minutesPerDay/p.Size)

	if p.Mixed > 0 {
		m.Note = strings.Repeat(" ", asciiLabelW) + "~ " + approximateText(p)
	}

	return m
}

// asciiLegend lists the states, wrapped into lines of at most width columns (the rows' width), each line indented like
// the rows; an entry wider than that gets a line of its own. Widths count runes, without the colour codes.
func asciiLegend(out *schedule.Output, cfg *config.Config, paint func(int, string) string, width int) string {
	const sep = "   "

	indent := strings.Repeat(" ", asciiLabelW)

	var (
		lines []string
		line  strings.Builder
		col   int
	)

	for i, t := range cfg.States {
		text := " " + diag.Sanitize(t.DisplayName)
		if strings.EqualFold(t.ID, out.Base) {
			text += " (base)"
		}

		w := len([]rune(cfg.Symbol(i))) + len([]rune(text))

		switch {
		case col == 0:
			line.WriteString(indent)
			col = asciiLabelW
		case col+len(sep)+w > width:
			lines = append(lines, line.String())
			line.Reset()
			line.WriteString(indent)
			col = asciiLabelW
		default:
			line.WriteString(sep)
			col += len(sep)
		}

		line.WriteString(paint(i, cfg.Symbol(i)) + text)
		col += w
	}

	return strings.Join(append(lines, line.String()), "\n")
}

// asciiRuler returns the label line (every six hours) and the tick line: + every six hours, | every hour, : every
// half hour when a bucket is at most 30 minutes. Pure ASCII, so it is single-width in every terminal.
func asciiRuler(size int) []string {
	cols := minutesPerDay / size
	labels := []rune(strings.Repeat(" ", cols))
	ticks := []rune(strings.Repeat(" ", cols))

	for c := range cols {
		switch mins := c * size; {
		case mins%(labelEvery*minutesPerHour) == 0:
			ticks[c] = '+'

			if c+labelLen <= cols {
				copy(labels[c:], []rune(fmt.Sprintf("%02d:00", mins/minutesPerHour)))
			}
		case mins%minutesPerHour == 0:
			ticks[c] = '|'
		case mins%halfHour == 0:
			ticks[c] = ':'
		}
	}

	pad := strings.Repeat(" ", asciiLabelW)

	return []string{pad + strings.TrimRight(string(labels), " "), pad + strings.TrimRight(string(ticks), " ")}
}

// painter colours text for state i, or returns it unchanged without color.
func painter(cfg *config.Config, color bool) func(int, string) string {
	if !color {
		return func(_ int, s string) string { return s }
	}

	bg, _ := parseHex(cfg.Representation.Theme.Dark.Background)

	return func(i int, s string) string {
		fg, ok := parseHex(cfg.Style(i).Color.Dark)
		if !ok {
			return s
		}

		r, g, b := blend(fg, bg)

		// The cell background carries the state colour; the symbol, drawn in the page background colour, stays as the
		// fallback for readers without colour.
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[38;2;%d;%d;%dm%s%s", r, g, b, bg[0], bg[1], bg[2], s, ansiReset)
	}
}

// parseHex reads #RRGGBB or #RRGGBBAA.
func parseHex(s string) (rgba, bool) {
	if len(s) != hexRGB && len(s) != hexRGBA || s[0] != '#' {
		return rgba{}, false
	}

	c := rgba{0, 0, 0, maxChannel}

	for i := 0; 1+2*i < len(s); i++ {
		v, err := strconv.ParseUint(s[1+2*i:3+2*i], 16, 8)
		if err != nil {
			return rgba{}, false
		}

		c[i] = int(v)
	}

	return c, true
}

// blend composites fg over the opaque bg, so translucent state colours look as they do in the SVG on a dark page.
func blend(fg, bg rgba) (int, int, int) {
	mix := func(f, b int) int { return (f*fg[3] + b*(maxChannel-fg[3]) + maxChannel/2) / maxChannel }

	return mix(fg[0], bg[0]), mix(fg[1], bg[1]), mix(fg[2], bg[2])
}
