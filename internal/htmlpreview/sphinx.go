package htmlpreview

import (
	"regexp"
	"strings"
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
