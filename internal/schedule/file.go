package schedule

import "strings"

// FileSuffix ends the name of a schedule file in a catalog (schedules/on-call.canned-schedule.md).
const FileSuffix = ".canned-schedule.md"

// FileStem derives a catalog file name, without FileSuffix, from a schedule's name: lower case, each run of spaces and
// hyphens one hyphen ("Night Shift" → "night-shift"). The name rule (ASCII letters, digits, spaces and hyphens, a
// letter first, a letter or digit last) makes the result safe as a file name: no slash, no dot, no leading hyphen.
func FileStem(name string) string {
	var b strings.Builder

	sep := false

	for _, r := range strings.ToLower(name) {
		if r == ' ' || r == '-' {
			sep = true

			continue
		}

		if sep && b.Len() > 0 {
			b.WriteByte('-')
		}

		sep = false

		b.WriteRune(r)
	}

	return b.String()
}

// FileName is FileStem with FileSuffix.
func FileName(name string) string { return FileStem(name) + FileSuffix }
