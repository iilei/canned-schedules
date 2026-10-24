// Package schedule turns a schedule document into the canonical weekly output.
//
// The stages (parse, normalise, paint) are not implemented yet; Evaluate returns ErrNotImplemented.
package schedule

import (
	"errors"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
)

// SchemaVersion is the schema version built into the binary. It applies when a document doesn't declare one.
const SchemaVersion = "1"

// ErrNotImplemented marks pipeline stages that don't exist yet.
var ErrNotImplemented = errors.New("not implemented")

// Options are the command-line settings that affect evaluation.
type Options struct {
	// Base is the --base tier; empty when not given.
	Base string
	// Granularity is the --granularity in minutes; every time must be a multiple of it.
	Granularity int
	// RequireBase is --require-base.
	RequireBase bool
}

// Output is the canonical output of a schedule document.
type Output struct {
	Annotations   map[string]string `json:"annotations"`
	SchemaVersion string            `json:"schema-version"`
	Name          string            `json:"name"`
	Hint          string            `json:"hint,omitempty"`
	Base          string            `json:"base"`
	Week          Week              `json:"week"`
}

// Week lists the runs of each day, Monday first. Field order defines the JSON key order.
type Week struct {
	MON []Run `json:"MON"`
	TUE []Run `json:"TUE"`
	WED []Run `json:"WED"`
	THU []Run `json:"THU"`
	FRI []Run `json:"FRI"`
	SAT []Run `json:"SAT"`
	SUN []Run `json:"SUN"`
}

// Run is a half-open [Start, End) stretch of one day with a single tier. End "00:00" means the end of the day.
type Run struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Tier  string `json:"tier"`
}

// Evaluate parses src and paints the week.
func Evaluate(_ []byte, _ *config.Config, _ Options) (*Output, diag.List, error) {
	return nil, nil, ErrNotImplemented
}
