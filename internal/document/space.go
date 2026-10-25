package document

import "strings"

// IsSpace reports whether r is whitespace in the schedule grammar: a space, a tab or a no-break space (U+00A0). The
// no-break space renders like a space and comes in through copy and paste from word processors (or Option+Space on a
// Mac), so wherever the grammar allows whitespace it counts as whitespace. Other invisible characters, such as a
// zero-width space, are not whitespace: they stay errors, escaped in messages.
func IsSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == ' '
}

// TrimSpace trims grammar whitespace and line breaks from both ends of s.
func TrimSpace(s string) string {
	return strings.TrimFunc(s, isTrimmed)
}

// TrimLeftSpace trims grammar whitespace and line breaks from the start of s.
func TrimLeftSpace(s string) string {
	return strings.TrimLeftFunc(s, isTrimmed)
}

func isTrimmed(r rune) bool {
	return IsSpace(r) || r == '\n' || r == '\r'
}
