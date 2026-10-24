package schedule

import (
	"strings"
	"testing"
)

func TestFinishRejectsInvalidOutput(t *testing.T) {
	t.Parallel()

	run := []Run{{Start: "00:00", End: "00:00", Tier: "Off"}}
	week := Week{MON: run, TUE: run, WED: run, THU: run, FRI: run, SAT: run, SUN: run}

	good := &Output{
		SchemaVersion: SchemaVersion, Name: "Plan", Base: "Off", Annotations: map[string]string{},
		Week: week, Transitions: []Transition{}, Warnings: []Warning{},
	}
	if err := good.Finish([]byte("# Plan\n")); err != nil {
		t.Fatalf("valid output rejected: %v", err)
	}

	if !strings.HasPrefix(good.Digests.Request, digestPrefix) || len(good.Digests.Effective) != len(digestPrefix)+64 {
		t.Errorf("digests = %+v", good.Digests)
	}

	bad := *good
	bad.Base = "1-not-a-tier-id"

	if err := bad.Finish(nil); err == nil {
		t.Error("an output with an invalid base must fail the output schema")
	}
}

func TestEffectiveDigestIgnoresPresentation(t *testing.T) {
	t.Parallel()

	run := []Run{{Start: "00:00", End: "00:00", Tier: "Off"}}
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
			"x-custom-owner": "ops",
		},
		Week:        week,
		Transitions: []Transition{},
		Warnings:    []Warning{},
	}
	c := *a
	c.Base = "Minimum"

	for _, o := range []*Output{a, b, &c} {
		if err := o.Finish([]byte(o.Name)); err != nil {
			t.Fatal(err)
		}
	}

	if a.Digests.Effective != b.Digests.Effective {
		t.Error("name, hint and annotations must not change the effective digest")
	}

	if a.Digests.Request == b.Digests.Request {
		t.Error("different input bytes must change the request digest")
	}

	if a.Digests.Effective == c.Digests.Effective {
		t.Error("a different base must change the effective digest, even when the week is the same")
	}
}
