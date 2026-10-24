// Package diag defines the diagnostics reported while reading flags, config and schedule documents.
package diag

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Severities.
const (
	Error Severity = iota + 1
	Warn
)

// Diagnostic codes. Golden fixtures pin these, so treat them as part of the contract.
const (
	CodeConfigDuplicateSymbol = "config-duplicate-symbol"
	CodeConfigDuplicateTier   = "config-duplicate-tier"
	CodeConfigSchema          = "config-schema"
	CodeDuplicateBlock        = "duplicate-block"
	CodeDuplicateName         = "duplicate-name"
	CodeDuplicateWeekday      = "duplicate-weekday"
	CodeEmptyTier             = "empty-tier"
	CodeInvalidFlag           = "invalid-flag"
	CodeInputFile             = "input-file"
	CodeInvalidArgs           = "invalid-args"
	CodeInvalidHint           = "invalid-hint"
	CodeInvalidName           = "invalid-name"
	CodeInvalidOverride       = "invalid-override"
	CodeInvalidRange          = "invalid-range"
	CodeInvalidTime           = "invalid-time"
	CodeInvalidWeekday        = "invalid-weekday"
	CodeMetaSchema            = "meta-schema"
	CodeMisplacedDirective    = "misplaced-directive"
	CodeMissingBase           = "missing-base"
	CodeMissingKind           = "missing-kind"
	CodeMissingName           = "missing-name"
	CodeMissingTitle          = "missing-title"
	CodeOutputSchema          = "output-schema"
	CodePreviewApproximate    = "preview-approximate"
	CodeRepeatedWeekday       = "repeated-weekday"
	CodeSampleInterval        = "sample-interval"
	CodeStyleReuse            = "style-reuse"
	CodeUnexpectedContent     = "unexpected-content"
	CodeUnknownKind           = "unknown-kind"
	CodeUnknownNamespace      = "unknown-namespace"
	CodeUnknownTier           = "unknown-tier"
)

// programName prefixes diagnostics that belong to no file.
const programName = "canned-schedules"

// Severity tells whether a diagnostic stops processing. It is a single byte, which keeps Diagnostic small enough to
// pass by value.
type Severity uint8

// Pos is a 1-based source position. Columns count runes. The zero value means "no position".
type Pos struct {
	Line int
	Col  int
}

// Diagnostic is a single error or warning.
type Diagnostic struct {
	Code string
	// Message is plain prose. Examples go into Example, never into the message.
	Message string
	// Example is optional: the exact text to type (Markdown, backticks included), e.g. "`21:45` to `00:00`".
	Example  string
	Pos      Pos
	Severity Severity
}

// List is an ordered collection of diagnostics.
type List []Diagnostic

// Errorf builds an error diagnostic.
func Errorf(pos Pos, code, format string, args ...any) Diagnostic {
	return Diagnostic{Severity: Error, Code: code, Message: fmt.Sprintf(format, args...), Pos: pos}
}

// Warnf builds a warning diagnostic.
func Warnf(pos Pos, code, format string, args ...any) Diagnostic {
	return Diagnostic{Severity: Warn, Code: code, Message: fmt.Sprintf(format, args...), Pos: pos}
}

// String returns "error" or "warn", the spelling used in every output.
func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warn:
		return "warn"
	default:
		return "unknown"
	}
}

// WithExample returns the diagnostic with an example of what to type instead.
func (d Diagnostic) WithExample(example string) Diagnostic {
	d.Example = example

	return d
}

// Golden renders the diagnostic in the golden fixture format: "<severity> <line>:<col> <code>".
// Diagnostics without a position use "-" instead of "<line>:<col>".
func (d Diagnostic) Golden() string {
	return d.Severity.String() + " " + d.where() + " " + d.Code
}

// Format renders the diagnostic for humans as "<file>:<line>:<col>: <severity>[<code>]: <message>", followed by
// "; e.g. <example>" when there is one. Without a position the prefix is just the file, and without a file it is the
// program name. File, message and example are sanitized.
func (d Diagnostic) Format(file string) string {
	return d.Location(file) + ": " + d.Severity.String() + "[" + d.Code + "]: " + d.Text()
}

// Text is the sanitized message plus "; e.g. <example>" when there is an example.
func (d Diagnostic) Text() string {
	if d.Example == "" {
		return Sanitize(d.Message)
	}

	return Sanitize(d.Message) + "; e.g. " + Sanitize(d.Example)
}

// Location returns "<file>:<line>:<col>", or just the file without a position, or the program name without a file.
// The file is sanitized.
func (d Diagnostic) Location(file string) string {
	if file == "" {
		return programName
	}

	file = Sanitize(file)
	if d.Pos == (Pos{}) {
		return file
	}

	return file + ":" + strconv.Itoa(d.Pos.Line) + ":" + strconv.Itoa(d.Pos.Col)
}

func (d Diagnostic) where() string {
	if d.Pos == (Pos{}) {
		return "-"
	}

	return strconv.Itoa(d.Pos.Line) + ":" + strconv.Itoa(d.Pos.Col)
}

// HasErrors reports whether the list contains at least one error.
func (l List) HasErrors() bool {
	for _, d := range l {
		if d.Severity == Error {
			return true
		}
	}

	return false
}

// Sort orders the list by position, keeping the original order for equal positions. Diagnostics without a position
// come first.
func (l List) Sort() {
	slices.SortStableFunc(l, func(a, b Diagnostic) int {
		return cmp.Or(cmp.Compare(a.Pos.Line, b.Pos.Line), cmp.Compare(a.Pos.Col, b.Pos.Col))
	})
}

// Sanitize makes s safe to echo into terminals, CI logs and reports: control characters (except tab), line and
// paragraph separators and bidirectional formatting characters become \uXXXX escapes. A newline in a file name could
// otherwise forge log lines (e.g. GitHub Actions "::" workflow commands), and bidi overrides can make text display
// differently from what it is ("Trojan Source").
func Sanitize(s string) string {
	if !strings.ContainsFunc(s, unsafeRune) {
		return s
	}

	var b strings.Builder

	for _, r := range s {
		if unsafeRune(r) {
			fmt.Fprintf(&b, "\\u%04x", r)

			continue
		}

		b.WriteRune(r)
	}

	return b.String()
}

func unsafeRune(r rune) bool {
	switch {
	case r == '\t':
		return false
	case unicode.IsControl(r), r == '\u2028', r == '\u2029':
		return true
	case r >= '\u202a' && r <= '\u202e', r >= '\u2066' && r <= '\u2069', r == '\u200e', r == '\u200f':
		return true
	default:
		return false
	}
}
