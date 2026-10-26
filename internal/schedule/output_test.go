package schedule

import (
	"testing"
)

func TestFinishRejectsInvalidOutput(t *testing.T) {
	t.Parallel()

	run := []Run{{Start: "00:00", End: "00:00", State: "Off"}}
	week := Week{MON: run, TUE: run, WED: run, THU: run, FRI: run, SAT: run, SUN: run}

	good := &Output{
		SchemaVersion: SchemaVersion, Name: "Plan", Base: "Off", Annotations: map[string]string{},
		Week: week, Transitions: []Transition{}, Warnings: []Warning{},
	}
	if err := good.Finish(); err != nil {
		t.Fatalf("valid output rejected: %v", err)
	}

	bad := *good
	bad.Base = "1-not-a-state-id"

	if err := bad.Finish(); err == nil {
		t.Error("an output with an invalid base must fail the output schema")
	}
}
