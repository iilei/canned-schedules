// Package render draws a schedule as SVG. Go computes a render model (every coordinate, colour, label and tooltip);
// the repo-baked template (templates/schedule-svg.tmpl) only emits markup. The template runs on the model decoded
// from JSON and uses no functions beyond the text/template builtins, so it also runs with gomplate.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/template"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/schedule"
	"github.com/iilei/canned-schedules/templates"
)

// Layout in pixels. The day grid matches the old CUE/gomplate renderer: 24 px per hour, 576 px per day.
const (
	fontFamily = "Ubuntu Mono, 'Courier New', Courier, monospace"

	margin       = 4
	labelW       = 48
	hourW        = 24
	titleH       = 34
	rulerH       = 44
	dayH         = 32
	blockH       = 24
	legendH      = 44
	columnGap    = 28
	lineH        = 18
	charW        = 7.5 // Monospace advance at font-size 12.
	headerCharW  = 8.5 // Monospace advance at font-size 14.
	titleCharW   = 11  // Monospace advance at font-size 18.
	cellPadding  = 18
	swatchW      = 20
	swatchGap    = 6
	legendGap    = 32
	labelEvery   = 6  // Hours between ruler labels.
	minMinorTick = 15 // Minutes; finer resolutions get no minor ticks (they would be unreadable).

	titleBaseline  = 12 // Distance of the title baseline above the ruler.
	dayLabelRaise  = 6  // Day labels sit this far above the bottom of their row.
	rulerLabelY    = 17 // Ruler label baseline below the top of the ruler.
	rulerTickBase  = 8  // Ticks end this far above the week.
	tickMinor      = 4
	tickHour       = 8
	tickLabeled    = 12
	tableFirstRow  = 2  // Rows start below the header line (1-based).
	swatchY        = 13 // Swatch top below the legend rule.
	swatchTextY    = 26 // Legend text baseline below the legend rule.
	pixelPrecision = 100

	minutesPerHour = 60
	hoursPerDay    = 24
	minutesPerDay  = hoursPerDay * minutesPerHour
	dayW           = hoursPerDay * hourW
)

var days = []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}

// Model is the render model the SVG template operates on. Strings are raw; the template escapes them.
type Model struct {
	Theme       config.Theme `json:"theme"`
	Font        string       `json:"font"`
	Transitions Transitions  `json:"transitions"`
	Labels      []Text       `json:"labels"`
	Tiers       []Tier       `json:"tiers"`
	Days        []Day        `json:"days"`
	Ticks       []Line       `json:"ticks"`
	Guides      []Line       `json:"guides"`
	Hint        Text         `json:"hint"`
	Title       Text         `json:"title"`
	Legend      Legend       `json:"legend"`
	Divider     Line         `json:"divider"`
	Width       float64      `json:"width"`
	Height      float64      `json:"height"`
	Resolution  int          `json:"resolution"`
}

// Tier carries the CSS class and colours of one configured tier.
type Tier struct {
	Class string `json:"class"`
	Light string `json:"light"`
	Dark  string `json:"dark"`
}

// Text is a positioned label.
type Text struct {
	Text string  `json:"text"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

// Line is a line segment.
type Line struct {
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
}

// Day is one row of the week.
type Day struct {
	Runs  []Run `json:"runs"`
	Label Text  `json:"label"`
}

// Run is one coloured block with its tooltip.
type Run struct {
	Class   string  `json:"class"`
	Tooltip string  `json:"tooltip"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
}

// Transitions is the table next to the week.
type Transitions struct {
	Empty   *Text    `json:"empty"`
	Headers []Text   `json:"headers"`
	Rows    [][]Text `json:"rows"`
}

// Legend is the rule above the legend and one swatch per configured tier.
type Legend struct {
	Swatches []Swatch `json:"swatches"`
	Rule     Line     `json:"rule"`
}

// Swatch is one legend entry.
type Swatch struct {
	Class string  `json:"class"`
	Text  Text    `json:"text"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

// SVG renders out as SVG using the tier order, display names and styles from cfg.
func SVG(out *schedule.Output, cfg *config.Config) ([]byte, error) {
	m := Build(out, cfg)

	raw, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encode render model: %w", err)
	}

	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode render model: %w", err)
	}

	t, err := template.New("svg").Parse(templates.ScheduleSVG)
	if err != nil {
		return nil, fmt.Errorf("parse svg template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render svg: %w", err)
	}

	return buf.Bytes(), nil
}

// Build computes the render model.
func Build(out *schedule.Output, cfg *config.Config) Model {
	m := Model{Theme: cfg.Representation.Theme, Font: fontFamily, Resolution: resolution(out)}

	for i := range cfg.Tiers {
		s := cfg.Style(i)
		m.Tiers = append(m.Tiers, Tier{Class: tierClass(i), Light: s.Color.Light, Dark: s.Color.Dark})
	}

	week := float64(titleH + rulerH)
	m.Title = Text{Text: out.Name, X: labelW, Y: titleH - titleBaseline}

	if out.Hint != "" {
		m.Hint = Text{
			Text: out.Hint, X: labelW + float64(len([]rune(out.Name)))*titleCharW + cellPadding,
			Y: titleH - titleBaseline,
		}
	}

	m.ruler(titleH)
	starts := transitionStarts(out.Transitions)

	for d, name := range days {
		y := week + float64(d*dayH)
		day := Day{Label: Text{Text: name, X: margin, Y: y + blockH - dayLabelRaise}}

		for _, r := range dayRuns(out, d) {
			from, to := minutes(r.Start), endMinutes(r.End)
			i, display := tierInfo(cfg, r.Tier)
			day.Runs = append(day.Runs, Run{
				Class:   tierClass(i),
				Tooltip: tooltip(display, name, r, starts[startKey(d, from, r.Tier)]),
				X:       labelW + px(from),
				Y:       y,
				Width:   px(to - from),
				Height:  blockH,
			})
		}

		m.Days = append(m.Days, day)
	}

	weekBottom := week + float64(len(days)*dayH)
	columnX := float64(labelW+dayW) + columnGap
	m.Divider = Line{X1: columnX - columnGap/2, Y1: titleH, X2: columnX - columnGap/2, Y2: weekBottom}

	tableBottom, right := m.table(out, cfg, columnX, week)
	right = math.Max(right, m.Hint.X+float64(len([]rune(m.Hint.Text)))*headerCharW)
	bottom := math.Max(weekBottom, tableBottom)
	right = m.legend(out, cfg, bottom, right)

	m.Width = math.Ceil(right + margin)
	m.Height = bottom + legendH
	m.Legend.Rule = Line{X1: labelW, Y1: bottom + margin, X2: m.Width - margin, Y2: bottom + margin}

	for i := range m.Guides {
		m.Guides[i].Y2 = weekBottom
	}

	return m
}

// ruler adds the time scale: labels every six hours, hour ticks, minor ticks at the resolution when it is readable,
// and faint guide lines through the week at every label.
func (m *Model) ruler(top float64) {
	base := top + rulerH - rulerTickBase
	week := top + rulerH

	for h := 0; h <= hoursPerDay; h++ {
		x := float64(labelW + h*hourW)
		tick := float64(tickHour)

		if h%labelEvery == 0 {
			tick = tickLabeled

			if h < hoursPerDay {
				m.Labels = append(m.Labels, Text{Text: fmt.Sprintf("%02d:00", h), X: x, Y: top + rulerLabelY})
			}

			m.Guides = append(m.Guides, Line{X1: x, Y1: week, X2: x})
		}

		m.Ticks = append(m.Ticks, Line{X1: x, Y1: base - tick, X2: x, Y2: base})
	}

	if m.Resolution >= minMinorTick && m.Resolution < minutesPerHour {
		for mins := m.Resolution; mins < minutesPerDay; mins += m.Resolution {
			if mins%minutesPerHour != 0 {
				x := labelW + px(mins)
				m.Ticks = append(m.Ticks, Line{X1: x, Y1: base - tickMinor, X2: x, Y2: base})
			}
		}
	}
}

// table adds the transition table and returns its bottom and right edge.
func (m *Model) table(out *schedule.Output, cfg *config.Config, x, top float64) (float64, float64) {
	cols := []string{"Transition", "POSIX cron", "AWS cron"}

	rows := make([][]string, 0, len(out.Transitions))
	for _, t := range out.Transitions {
		_, display := tierInfo(cfg, t.Tier)
		rows = append(rows, []string{compactDays(t.Days) + " " + t.Time + " → " + display, t.Cron.POSIX, t.Cron.AWS})
	}

	widths := make([]float64, len(cols))
	for c, h := range cols {
		widths[c] = float64(len([]rune(h))) * headerCharW

		for _, r := range rows {
			widths[c] = math.Max(widths[c], float64(len([]rune(r[c])))*charW)
		}
	}

	colX := x
	for c, h := range cols {
		m.Transitions.Headers = append(m.Transitions.Headers, Text{Text: h, X: colX, Y: top + lineH})
		colX += widths[c] + cellPadding
	}

	for i, r := range rows {
		y := top + float64(i+tableFirstRow)*lineH
		colX = x
		row := make([]Text, 0, len(r))

		for c, cell := range r {
			row = append(row, Text{Text: cell, X: colX, Y: y})
			colX += widths[c] + cellPadding
		}

		m.Transitions.Rows = append(m.Transitions.Rows, row)
	}

	if len(rows) == 0 {
		m.Transitions.Empty = &Text{Text: "No transitions: one tier all week", X: x, Y: top + tableFirstRow*lineH}
	}

	return top + float64(max(len(rows), 1)+1)*lineH + margin, colX - cellPadding
}

// legend adds one swatch per configured tier and returns the new right edge.
func (m *Model) legend(out *schedule.Output, cfg *config.Config, top, right float64) float64 {
	x := float64(labelW)

	for i, t := range cfg.Tiers {
		label := diag.Sanitize(t.DisplayName)
		if strings.EqualFold(t.ID, out.Base) {
			label += " (base)"
		}

		m.Legend.Swatches = append(m.Legend.Swatches, Swatch{
			Class: tierClass(i),
			X:     x,
			Y:     top + swatchY,
			Text:  Text{Text: label, X: x + swatchW + swatchGap, Y: top + swatchTextY},
		})
		x += swatchW + swatchGap + float64(len([]rune(label)))*charW + legendGap
	}

	return math.Max(right, x-legendGap)
}

// resolution is the grid resolution in minutes: the GCD of every run boundary and 60, so cells divide the hour.
func resolution(out *schedule.Output) int {
	g := minutesPerHour

	for d := range days {
		for _, r := range dayRuns(out, d) {
			g = gcd(g, minutes(r.Start))
			g = gcd(g, minutes(r.End))
		}
	}

	return g
}

func dayRuns(out *schedule.Output, d int) []schedule.Run {
	w := out.Week

	return [][]schedule.Run{w.MON, w.TUE, w.WED, w.THU, w.FRI, w.SAT, w.SUN}[d]
}

// transitionStarts maps day/minute/tier to the transition that starts a run there.
func transitionStarts(ts []schedule.Transition) map[string]*schedule.Transition {
	starts := map[string]*schedule.Transition{}

	for i := range ts {
		for _, d := range ts[i].Days {
			for n, name := range days {
				if name == d {
					starts[startKey(n, minutes(ts[i].Time), ts[i].Tier)] = &ts[i]
				}
			}
		}
	}

	return starts
}

func startKey(day, minute int, tier string) string {
	return strconv.Itoa(day) + "/" + strconv.Itoa(minute) + "/" + tier
}

func tooltip(display, day string, r schedule.Run, start *schedule.Transition) string {
	tip := display + " · " + day + " " + r.Start + "–" + r.End
	if start != nil {
		tip += "\nstarts: posix " + start.Cron.POSIX + " · aws " + start.Cron.AWS
	}

	return tip
}

// tierInfo returns the configured index and sanitized display name of a tier id.
func tierInfo(cfg *config.Config, id string) (int, string) {
	for i, t := range cfg.Tiers {
		if strings.EqualFold(t.ID, id) {
			return i, diag.Sanitize(t.DisplayName)
		}
	}

	return 0, diag.Sanitize(id)
}

// compactDays renders a Monday-first day list as ranges and lists: MON-FRI, SAT-SUN, MON,WED.
func compactDays(list []string) string {
	idx := make([]int, 0, len(list))

	for _, d := range list {
		for n, name := range days {
			if name == d {
				idx = append(idx, n)
			}
		}
	}

	var parts []string

	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && idx[j+1] == idx[j]+1 {
			j++
		}

		part := days[idx[i]]
		if j > i {
			part += "-" + days[idx[j]]
		}

		parts = append(parts, part)
		i = j + 1
	}

	return strings.Join(parts, ",")
}

func tierClass(i int) string {
	return "tier-" + strconv.Itoa(i)
}

func minutes(hhmm string) int {
	h, m, _ := strings.Cut(hhmm, ":")
	hours, _ := strconv.Atoi(h)
	mins, _ := strconv.Atoi(m)

	return hours*minutesPerHour + mins
}

// endMinutes treats a run end of "00:00" as the end of the day.
func endMinutes(hhmm string) int {
	if m := minutes(hhmm); m > 0 {
		return m
	}

	return minutesPerDay
}

func px(mins int) float64 {
	return math.Round(float64(mins)*hourW/minutesPerHour*pixelPrecision) / pixelPrecision
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}

	return a
}
