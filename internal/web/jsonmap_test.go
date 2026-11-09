package web

import (
	"strings"
	"testing"

	"github.com/iilei/canned-schedules/internal/schedule"
)

func TestJSONSpans(t *testing.T) {
	t.Parallel()

	text := "{\n  \"name\": \"Plan\",\n  \"annotations\": {\n    \"x-owner\": \"team\"\n  },\n" +
		"  \"week\": {\n    \"MON\": [\n      {\n        \"start\": \"00:00\"\n      }\n    ]\n  }\n}\n"

	spans, err := jsonSpans(text)
	if err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]schedule.Lines{
		"/name":                {From: 2, To: 2},
		"/annotations":         {From: 3, To: 5},
		"/annotations/x-owner": {From: 4, To: 4},
		"/week/MON/0":          {From: 8, To: 10},
		"/week/MON/0/start":    {From: 9, To: 9},
	} {
		if got := spans[path]; got != want {
			t.Errorf("%s = %+v, want %+v", path, got, want)
		}
	}
}

func TestHighlightsCarryJSON(t *testing.T) {
	t.Parallel()

	src := "# Plan\n\n> [!SCHEDULE-META] Late Fridays\n> - x-owner: `team`\n\n> [!SCHEDULE-BASE] Minimum\n\n" +
		"> [!SCHEDULE-STATE] Maximum\n> - `20:00` to `02:00` FRI\n"

	res := Evaluate([]byte(src), nil, nil)
	lines := strings.Split(res.JSON, "\n")
	byKind := map[string][]string{}

	for _, h := range res.Highlights {
		if len(h.JSON) == 0 {
			t.Errorf("%s at line %d has no JSON spans", h.Kind, h.Line)
		}

		for _, s := range h.JSON {
			byKind[h.Kind] = append(byKind[h.Kind], strings.Join(lines[s.From-1:s.To], "\n"))
		}
	}

	for kind, want := range map[string]string{
		"name": `"name": "Plan"`, "meta": `"x-owner": "team"`,
		// The marker covers summary and the whole annotations object, closing brace included.
		"meta-block": "\"summary\": \"Late Fridays\",\n  \"annotations\": {\n    \"x-owner\": \"team\"\n  },",
		"base":       `"base": "Minimum"`, "range": `"state": "Maximum"`,
	} {
		if got := strings.Join(byKind[kind], "\n"); !strings.Contains(got, want) {
			t.Errorf("%s spans = %q, want %q in them", kind, got, want)
		}
	}

	// The range's transition (FRI 20:00 → Maximum) is part of its spans.
	if !strings.Contains(strings.Join(byKind["range"], "\n"), `"time": "20:00"`) {
		t.Errorf("range spans miss the transition: %q", byKind["range"])
	}

	// The page's SVG shows the annotations section.
	if !strings.Contains(res.SVG, `class="annotations"`) || !strings.Contains(res.SVG, ">team</text>") {
		t.Error("the page's SVG must list the annotations")
	}
}
