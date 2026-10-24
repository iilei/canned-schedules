package document

import (
	"sort"
	"unicode/utf8"

	"github.com/iilei/canned-schedules/internal/diag"
)

// lineIndex maps byte offsets to 1-based line and rune columns.
type lineIndex struct {
	src    []byte
	starts []int
}

func newLineIndex(src []byte) lineIndex {
	starts := []int{0}

	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}

	return lineIndex{src: src, starts: starts}
}

func (l lineIndex) pos(offset int) diag.Pos {
	line := sort.SearchInts(l.starts, offset+1) - 1
	start := l.starts[line]

	return diag.Pos{Line: line + 1, Col: utf8.RuneCount(l.src[start:offset]) + 1}
}
