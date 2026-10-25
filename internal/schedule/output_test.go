package schedule

import (
	"strings"
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

	if !strings.HasPrefix(good.Digests.Schedule, digestPrefix) || len(good.Digests.Schedule) != len(digestPrefix)+64 {
		t.Errorf("digests = %+v", good.Digests)
	}

	bad := *good
	bad.Base = "1-not-a-state-id"

	if err := bad.Finish(); err == nil {
		t.Error("an output with an invalid base must fail the output schema")
	}
}

func TestScheduleDigestIgnoresPresentation(t *testing.T) {
	t.Parallel()

	run := []Run{{Start: "00:00", End: "00:00", State: "Off"}}
	week := Week{MON: run, TUE: run, WED: run, THU: run, FRI: run, SAT: run, SUN: run}
	a := &Output{
		SchemaVersion: SchemaVersion, Name: "Plan A", Base: "Off", Annotations: map[string]string{},
		Week: week, Transitions: []Transition{}, Warnings: []Warning{},
	}
	b := &Output{
		SchemaVersion: SchemaVersion,
		Name:          "Plan B",
		Hint:          "Other words",
		Base:          "Off",
		Annotations: map[string]string{
			"x-owner": "ops",
		},
		Week:        week,
		Transitions: []Transition{},
		Warnings:    []Warning{},
	}
	c := *a
	c.Base = "Minimum"

	for _, o := range []*Output{a, b, &c} {
		if err := o.Finish(); err != nil {
			t.Fatal(err)
		}
	}

	if a.Digests.Schedule != b.Digests.Schedule {
		t.Error("name, hint and annotations must not change the schedule digest")
	}

	if a.Digests.Schedule == c.Digests.Schedule {
		t.Error("a different base must change the schedule digest, even when the week is the same")
	}
}
