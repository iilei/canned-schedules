// Package diag defines the diagnostics reported while reading flags, config and schedule documents.
package diag

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
)

// Severities.
const (
	Error Severity = "error"
	Warn  Severity = "warn"
)

// Diagnostic codes. Golden fixtures pin these, so treat them as part of the contract.
const (
	CodeConfigDuplicateSymbol = "config-duplicate-symbol"
	CodeConfigDuplicateTier   = "config-duplicate-tier"
	CodeConfigSchema          = "config-schema"
	CodeDuplicateBlock        = "duplicate-block"
	CodeDuplicateName         = "duplicate-name"
	CodeEmptyTier             = "empty-tier"
	CodeGranularity           = "granularity"
	CodeInvalidFlag           = "invalid-flag"
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
	CodeStyleReuse            = "style-reuse"
	CodeUnexpectedContent     = "unexpected-content"
	CodeUnknownKind           = "unknown-kind"
	CodeUnknownNamespace      = "unknown-namespace"
	CodeUnknownTier           = "unknown-tier"
)

// Severity tells whether a diagnostic stops processing.
type Severity string

// Pos is a 1-based source position. Columns count runes. The zero value means "no position".
type Pos struct {
	Line int
	Col  int
}

// Diagnostic is a single error or warning.
type Diagnostic struct {
	Severity Severity
	Code     string
	Message  string
	Pos      Pos
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

// Golden renders the diagnostic in the golden fixture format: "<severity> <line>:<col> <code>".
// Diagnostics without a position use "-" instead of "<line>:<col>".
func (d Diagnostic) Golden() string {
	return string(d.Severity) + " " + d.where() + " " + d.Code
}

// Format renders the diagnostic for humans, prefixed with the file name.
func (d Diagnostic) Format(file string) string {
	if d.Pos == (Pos{}) {
		return fmt.Sprintf("%s: %s[%s]: %s", file, d.Severity, d.Code, d.Message)
	}

	return fmt.Sprintf("%s:%d:%d: %s[%s]: %s", file, d.Pos.Line, d.Pos.Col, d.Severity, d.Code, d.Message)
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
