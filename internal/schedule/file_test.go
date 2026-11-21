package schedule

import (
	"regexp"
	"testing"
)

func TestFileStem(t *testing.T) {
	t.Parallel()

	// The stem pattern of site.schema.json's catalog names.
	stem := regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

	for name, want := range map[string]string{
		"Office Hours":     "office-hours",
		"Night Shift":      "night-shift",
		"On-Call Roster 2": "on-call-roster-2",
		"A  -  B":          "a-b",
		"Q4-Peak":          "q4-peak",
		"X":                "x",
		"ALLCAPS":          "allcaps",
	} {
		got := FileStem(name)
		if got != want || !stem.MatchString(got) {
			t.Errorf("FileStem(%q) = %q, want %q", name, got, want)
		}
	}

	if got := FileName("Night Shift"); got != "night-shift.canned-schedule.md" {
		t.Errorf("FileName = %q", got)
	}
}
