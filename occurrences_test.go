package cannedschedules_test

import (
	"testing"

	cannedschedules "github.com/iilei/canned-schedules"
)

// An output stored before occurrences existed still renders.
func TestRenderSVGWithoutOccurrences(t *testing.T) {
	src := "# Rota\n\n> [!SCHEDULE-BASE] Off\n\n> [!SCHEDULE-STATE] Medium\n> - `06:00` to `14:00` MON,TUE\n"

	r, err := cannedschedules.Evaluate([]byte(src), nil)
	if err != nil || !r.Valid() {
		t.Fatalf("evaluate: %v, %v", err, r.Diagnostics)
	}

	if got := len(r.Output.Occurrences); got != 4 {
		t.Fatalf("occurrences: got %d, want 4", got)
	}

	stored := *r.Output
	stored.Occurrences = nil

	if _, err := cannedschedules.RenderSVG(&stored, nil, cannedschedules.SVGOptions{}); err != nil {
		t.Errorf("render: %v", err)
	}
}
