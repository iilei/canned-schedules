package htmlpreview

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/iilei/canned-schedules/internal/config"
)

const scheduleStart = `<div class="admonition schedule`

var tag = regexp.MustCompile(`<[^>]*>`)

// abbrWeekdays wraps weekday names inside schedule admonitions in <abbr title="Monday">MON</abbr>. Text inside tags
// and code spans is left alone.
func abbrWeekdays(html string) string {
	var out strings.Builder

	for {
		start := strings.Index(html, scheduleStart)
		if start < 0 {
			out.WriteString(html)

			return out.String()
		}

		end := strings.Index(html[start:], "</div>")
		if end < 0 {
			out.WriteString(html)

			return out.String()
		}

		end += start
		out.WriteString(html[:start])
		out.WriteString(abbrText(html[start:end]))
		html = html[end:]
	}
}

func abbrText(fragment string) string {
	var (
		out    strings.Builder
		inCode bool
		last   int
	)

	for _, loc := range tag.FindAllStringIndex(fragment, -1) {
		out.WriteString(abbrRun(fragment[last:loc[0]], inCode))

		t := fragment[loc[0]:loc[1]]
		switch {
		case strings.HasPrefix(t, "<code"):
			inCode = true
		case t == "</code>":
			inCode = false
		}

		out.WriteString(t)
		last = loc[1]
	}

	out.WriteString(abbrRun(fragment[last:], inCode))

	return out.String()
}

func abbrRun(s string, inCode bool) string {
	if inCode {
		return s
	}

	return weekday.ReplaceAllStringFunc(s, func(d string) string {
		return `<abbr title="` + weekdayNames[d] + `">` + d + `</abbr>`
	})
}

// StyleSheet returns CSS for the Sphinx preview's schedule admonitions, coloured from the config's tier styles. The
// container carries data-theme="light", "dark" or "system"; system follows prefers-color-scheme.
func StyleSheet(cfg *config.Config, scope string) string {
	var b strings.Builder

	for i := range cfg.Tiers {
		c := cfg.Style(i).Color
		sel := fmt.Sprintf("%s .admonition.tier-%d", scope, i)
		fmt.Fprintf(&b, "%s { --tier: %s; }\n", sel, c.Light)
		fmt.Fprintf(&b, "[data-theme=\"dark\"] %s { --tier: %s; }\n", sel, c.Dark)
		fmt.Fprintf(
			&b,
			"@media (prefers-color-scheme: dark) { [data-theme=\"system\"] %s { --tier: %s; } }\n",
			sel,
			c.Dark,
		)
	}

	return b.String()
}
