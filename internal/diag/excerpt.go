package diag

import "strings"

// excerptFence is the start of the fence the Markdown report template uses. A source line starting with it could
// close the fence early, so such lines get no excerpt.
const excerptFence = "~~~"

// Excerpt is the source line of a diagnostic and a marker line with a caret under the column.
type Excerpt struct {
	Text   string `json:"text"`
	Marker string `json:"marker"`
}

// SourceLines splits src into lines for MakeExcerpt.
func SourceLines(src []byte) []string {
	return strings.Split(string(src), "\n")
}

// MakeExcerpt returns the sanitized source line at pos with a caret under the column, or nil when pos has no line.
// Tabs before the column are copied into the marker so the caret lines up.
func MakeExcerpt(lines []string, pos Pos) *Excerpt {
	if pos.Line < 1 || pos.Line > len(lines) {
		return nil
	}

	line := strings.TrimRight(lines[pos.Line-1], "\r")
	if strings.HasPrefix(strings.TrimLeft(line, " \t"), excerptFence) {
		return nil
	}

	runes := []rune(line)
	before := string(runes[:min(max(pos.Col-1, 0), len(runes))])

	var marker strings.Builder
	for _, r := range Sanitize(before) {
		if r == '\t' {
			marker.WriteRune('\t')
		} else {
			marker.WriteRune(' ')
		}
	}

	marker.WriteRune('^')

	return &Excerpt{Text: Sanitize(line), Marker: marker.String()}
}
