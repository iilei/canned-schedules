// Package render draws the schedule previews: the SVG week and the ASCII preview, both from the same downsampled week
// (preview buckets, see Downsample). Go computes a render model (every coordinate, colour, label and tooltip); the
// repo-baked templates (templates/schedule-svg.tmpl, templates/schedule-ascii.tmpl) only emit markup. The template runs on the model decoded
// from JSON and uses no functions beyond the text/template builtins, so it also runs with gomplate.
package render

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/schedule"
	"github.com/iilei/canned-schedules/templates"
)

// SVG themes (--svg-theme).
const (
	ThemeAuto  = "auto"
	ThemeLight = "light"
	ThemeDark  = "dark"
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
	sectionGap     = 12 // Space above and below the rule under the META section.
	tableLead      = 2  // The table's header baseline sits lineH below its top; this pulls it closer to the rule.
	pixelPrecision = 100

	// SourceNamespace is the XML namespace of the source element in an SVG's metadata.
	SourceNamespace = "https://github.com/iilei/canned-schedules#svg-source"

	minutesPerHour = 60
	hoursPerDay    = 24
	minutesPerDay  = hoursPerDay * minutesPerHour
	dayW           = hoursPerDay * hourW
)

var days = []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}

// Model is the render model the SVG template operates on. Strings are raw; the template escapes them.
type Model struct {
	// Dark is the palette for prefers-color-scheme: dark; only set in auto mode.
	Dark *Palette `json:"dark"`
	// Source is the request embedded in the metadata (--svg-embed-source); nil means none.
	Source *Source `json:"source"`
	// Annotations is the META section above the transition table (--svg-annotations); nil means none.
	Annotations *Annotations `json:"annotations"`
	Palette     Palette      `json:"palette"`
	Scheme      string       `json:"scheme"`
	Font        string       `json:"font"`
	Transitions Transitions  `json:"transitions"`
	Labels      []Text       `json:"labels"`
	Days        []Day        `json:"days"`
	Ticks       []Line       `json:"ticks"`
	Guides      []Line       `json:"guides"`
	Summary     Text         `json:"summary"`
	Title       Text         `json:"title"`
	Legend      Legend       `json:"legend"`
	Divider     Line         `json:"divider"`
	Width       float64      `json:"width"`
	Height      float64      `json:"height"`
	Bucket      int          `json:"bucket"`
}

// Source is the canonical request an SVG carries in its metadata: evaluating it regenerates the JSON, so the picture
// stays traceable to its schedule.
type Source struct {
	Namespace string `json:"namespace"`
	MediaType string `json:"mediaType"`
	Generator string `json:"generator"`
	Text      string `json:"text"`
}

// SVGOptions configure the SVG beyond the preview buckets.
type SVGOptions struct {
	// Source is embedded in the metadata when non-nil; its Namespace and MediaType are filled in by SVG.
	Source *Source
	// Theme is ThemeAuto (follow the viewer's prefers-color-scheme), ThemeLight or ThemeDark.
	Theme string
	// Annotations shows the META section (summary and x-* keys) above the transition table. Opt-in, because owners
	// and links would otherwise travel with every shared picture.
	Annotations bool
}

// Annotations is the META section above the transition table: the summary (moved there from beside the title, so
// it sits with the rest of META) and the annotations sorted by key as in the JSON, then a rule above the table.
type Annotations struct {
	// Summary is nil without one.
	Summary *Text           `json:"summary"`
	Rows    []AnnotationRow `json:"rows"`
	Heading Text            `json:"heading"`
	Rule    Line            `json:"rule"`
}

// AnnotationRow is one annotation: the key and its value, in two aligned columns.
type AnnotationRow struct {
	Key   Text `json:"key"`
	Value Text `json:"value"`
}

// State carries the CSS class and colour of one configured state.
type State struct {
	Class string `json:"class"`
	Fill  string `json:"fill"`
}

// Palette holds the page and state colours of one colour scheme.
type Palette struct {
	Background string  `json:"background"`
	Label      string  `json:"label"`
	States     []State `json:"states"`
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

// Legend is the rule above the legend and one swatch per configured state.
type Legend struct {
	Note     *Text    `json:"note"`
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

// SVG renders out as SVG with buckets of size minutes, using the state order, display names and styles from cfg.
func SVG(out *schedule.Output, cfg *config.Config, size int, o SVGOptions) ([]byte, error) {
	m := Build(out, cfg, Downsample(out, cfg, size), o)

	if o.Source != nil {
		s := *o.Source
		s.Namespace, s.MediaType = SourceNamespace, "text/markdown"
		m.Source = &s
	}

	return execute(templates.ScheduleSVG, m)
}

// execute renders a preview template; see templates.Execute.
func execute(tmpl string, model any) ([]byte, error) {
	return templates.Execute(tmpl, model)
}

// Build computes the render model for the downsampled week p; o.Source is ignored (SVG embeds it).
func Build(out *schedule.Output, cfg *config.Config, p Preview, o SVGOptions) Model {
	m := Model{Font: fontFamily, Bucket: p.Size}

	switch o.Theme {
	case ThemeLight:
		m.Palette, m.Scheme = palette(cfg, false), "light"
	case ThemeDark:
		m.Palette, m.Scheme = palette(cfg, true), "dark"
	default:
		dark := palette(cfg, true)
		m.Palette, m.Dark, m.Scheme = palette(cfg, false), &dark, "light dark"
	}

	week := float64(titleH + rulerH)
	m.Title = Text{Text: out.Name, X: labelW, Y: titleH - titleBaseline}

	section := o.Annotations && (len(out.Annotations) > 0 || out.Summary != "")

	if out.Summary != "" && !section {
		m.Summary = Text{
			Text: out.Summary, X: labelW + float64(len([]rune(out.Name)))*titleCharW + cellPadding,
			Y: titleH - titleBaseline,
		}
	}

	m.ruler(titleH)
	starts := transitionStarts(out.Transitions)

	for d, name := range days {
		y := week + float64(d*dayH)
		day := Day{Label: Text{Text: name, X: margin, Y: y + blockH - dayLabelRaise}}

		for _, sg := range p.segments(d) {
			i, display := stateInfo(cfg, sg.state)
			day.Runs = append(day.Runs, Run{
				Class:   stateClass(i),
				Tooltip: tooltip(display, name, sg, starts[startKey(d, sg.from, sg.state)]),
				X:       labelW + px(sg.from),
				Y:       y,
				Width:   px(sg.to - sg.from),
				Height:  blockH,
			})
		}

		m.Days = append(m.Days, day)
	}

	weekBottom := week + float64(len(days)*dayH)
	columnX := float64(labelW+dayW) + columnGap
	m.Divider = Line{X1: columnX - columnGap/2, Y1: titleH, X2: columnX - columnGap/2, Y2: weekBottom}

	tableTop := week
	if section {
		tableTop = m.annotations(out, columnX, titleH)
	}

	tableBottom, right := m.table(out, cfg, columnX, tableTop)
	right = math.Max(right, m.annotationsRight())
	right = math.Max(right, m.Summary.X+float64(len([]rune(m.Summary.Text)))*headerCharW)
	bottom := math.Max(weekBottom, tableBottom)
	right = m.legend(out, cfg, p, bottom, right)
	m.Width = math.Ceil(right + margin)
	m.Height = bottom + legendH
	m.Legend.Rule = Line{X1: labelW, Y1: bottom + margin, X2: m.Width - margin, Y2: bottom + margin}

	if m.Annotations != nil {
		m.Annotations.Rule.X2 = m.Width - margin
	}

	m.Divider.Y2 = bottom

	for i := range m.Guides {
		m.Guides[i].Y2 = weekBottom
	}

	return m
}

// palette collects the light or dark colours of the theme and the state styles.
func palette(cfg *config.Config, dark bool) Palette {
	surface := cfg.Representation.Theme.Light
	if dark {
		surface = cfg.Representation.Theme.Dark
	}

	p := Palette{Background: surface.Background, Label: surface.Label}

	for i := range cfg.States {
		c := cfg.Style(i).Color
		fill := c.Light

		if dark {
			fill = c.Dark
		}

		p.States = append(p.States, State{Class: stateClass(i), Fill: fill})
	}

	return p
}

// ruler adds the time scale: labels every six hours, hour ticks, minor ticks at the bucket size when it is readable,
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

	if m.Bucket >= minMinorTick && m.Bucket < minutesPerHour {
		for mins := m.Bucket; mins < minutesPerDay; mins += m.Bucket {
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
		_, display := stateInfo(cfg, t.State)
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
		m.Transitions.Empty = &Text{Text: "No transitions: one state all week", X: x, Y: top + tableFirstRow*lineH}
	}

	return top + float64(max(len(rows), 1)+1)*lineH + margin, colX - cellPadding
}

// legend adds one swatch per configured state, a note when the preview is approximate, and returns the new right
// edge.
func (m *Model) legend(out *schedule.Output, cfg *config.Config, p Preview, top, right float64) float64 {
	x := float64(labelW)

	for i, t := range cfg.States {
		label := diag.Sanitize(t.DisplayName)
		if strings.EqualFold(t.ID, out.Base) {
			label += " (base)"
		}

		m.Legend.Swatches = append(m.Legend.Swatches, Swatch{
			Class: stateClass(i),
			X:     x,
			Y:     top + swatchY,
			Text:  Text{Text: label, X: x + swatchW + swatchGap, Y: top + swatchTextY},
		})
		x += swatchW + swatchGap + float64(len([]rune(label)))*charW + legendGap
	}

	if p.Mixed > 0 {
		note := approximateNote(p)
		m.Legend.Note = &Text{Text: note, X: x, Y: top + swatchTextY}
		x += float64(len([]rune(note)))*charW + legendGap
	}

	return math.Max(right, x-legendGap)
}

// annotations adds the META section at x from top: a heading, the summary, one key/value row per annotation (sorted
// by key like the JSON), then a rule with a margin on both sides. It returns where the transition table starts.
func (m *Model) annotations(out *schedule.Output, x, top float64) float64 {
	keys := make([]string, 0, len(out.Annotations))
	keyW := 0

	for k := range out.Annotations {
		keys = append(keys, k)
		keyW = max(keyW, len([]rune(k)))
	}

	slices.Sort(keys)

	y := top + lineH
	a := &Annotations{Heading: Text{Text: "META", X: x, Y: y}}

	if out.Summary != "" {
		y += lineH
		a.Summary = &Text{Text: diag.Sanitize(out.Summary), X: x, Y: y}
	}

	valueX := x + float64(keyW)*charW + cellPadding

	for _, k := range keys {
		y += lineH
		a.Rows = append(a.Rows, AnnotationRow{
			Key:   Text{Text: diag.Sanitize(k), X: x, Y: y},
			Value: Text{Text: diag.Sanitize(out.Annotations[k]), X: valueX, Y: y},
		})
	}

	rule := y + sectionGap
	a.Rule = Line{X1: x, Y1: rule, Y2: rule} // X2 follows the final width.
	m.Annotations = a

	return rule + sectionGap - tableLead
}

// annotationsRight is the right edge of the META section's text; 0 without one.
func (m *Model) annotationsRight() float64 {
	a := m.Annotations
	if a == nil {
		return 0
	}

	right := a.Heading.X + float64(len([]rune(a.Heading.Text)))*headerCharW
	if a.Summary != nil {
		right = math.Max(right, a.Summary.X+float64(len([]rune(a.Summary.Text)))*headerCharW)
	}

	for _, r := range a.Rows {
		right = math.Max(right, r.Value.X+float64(len([]rune(r.Value.Text)))*charW)
	}

	return right
}

// approximateNote explains a lossy preview in the SVG.
func approximateNote(p Preview) string {
	return "≈ " + approximateText(p)
}

// approximateText is the shared wording; the ASCII preview prefixes it with "~" to stay pure ASCII.
func approximateText(p Preview) string {
	return fmt.Sprintf("%d-minute buckets, %d show their majority state", p.Size, p.Mixed)
}

func dayRuns(out *schedule.Output, d int) []schedule.Run {
	w := out.Week

	return [][]schedule.Run{w.MON, w.TUE, w.WED, w.THU, w.FRI, w.SAT, w.SUN}[d]
}

// transitionStarts maps day/minute/state to the transition that starts a run there.
func transitionStarts(ts []schedule.Transition) map[string]*schedule.Transition {
	starts := map[string]*schedule.Transition{}

	for i := range ts {
		for _, d := range ts[i].Days {
			for n, name := range days {
				if name == d {
					starts[startKey(n, minutes(ts[i].Time), ts[i].State)] = &ts[i]
				}
			}
		}
	}

	return starts
}

func startKey(day, minute int, state string) string {
	return strconv.Itoa(day) + "/" + strconv.Itoa(minute) + "/" + state
}

func tooltip(display, day string, sg segment, start *schedule.Transition) string {
	tip := display + " · " + day + " " + clock(sg.from) + "–" + clock(sg.to%minutesPerDay)
	if start != nil {
		tip += "\nstarts: posix " + start.Cron.POSIX + " · aws " + start.Cron.AWS
	}

	return tip
}

func clock(mins int) string {
	return fmt.Sprintf("%02d:%02d", mins/minutesPerHour, mins%minutesPerHour)
}

// stateInfo returns the configured index and sanitized display name of a state id.
func stateInfo(cfg *config.Config, id string) (int, string) {
	for i, t := range cfg.States {
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

func stateClass(i int) string {
	return "state-" + strconv.Itoa(i)
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
