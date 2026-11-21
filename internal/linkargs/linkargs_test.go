package linkargs

import (
	"strings"
	"testing"
)

func TestConflicts(t *testing.T) {
	t.Parallel()

	site := []string{
		"--require-base", "true", "--error-unless-alert-pattern=(?i)NOTE", "--set-config-at", "states",
		"--set-config-value", "[]", "--sample-interval", "15",
	}

	for name, c := range map[string]struct {
		link []string
		want string
	}{
		"same flag":     {[]string{"--require-base", "false"}, "--require-base is set"},
		"inline":        {[]string{"--error-unless-alert-pattern=.*"}, "--error-unless-alert-pattern is set"},
		"same path":     {[]string{"--set-config-at", "states", "--set-config-value", "[]"}, "sets states"},
		"inside":        {[]string{"--set-config-at", "states[0]", "--set-config-value", "{}"}, "sets states"},
		"other path":    {[]string{"--set-config-at", "representation.ascii", "--set-config-value", "{}"}, ""},
		"similar name":  {[]string{"--set-config-at", "statesx", "--set-config-value", "{}"}, ""},
		"other flag":    {[]string{"--base", "Medium"}, ""},
		"no link flags": {nil, ""},
	} {
		diags := Conflicts(site, c.link)

		var got []string
		for _, d := range diags {
			got = append(got, d.Message)
		}

		joined := strings.Join(got, "\n")
		if (c.want == "") != (len(diags) == 0) || !strings.Contains(joined, c.want) {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}

	// Inside works both ways: a site that sets one style conflicts with a link replacing all of them.
	if d := Conflicts([]string{"--set-config-at", "representation.styles.states[0]", "--set-config-value", "{}"},
		[]string{"--set-config-at", "representation.styles", "--set-config-value", "{}"}); len(d) != 1 {
		t.Errorf("parent path: %v", d)
	}
}

func TestCheckSite(t *testing.T) {
	t.Parallel()

	if d := CheckSite([]string{"--sample-interval", "15", "--base=Medium"}); len(d) != 0 {
		t.Errorf("allowed flags refused: %v", d)
	}

	if d := Check(
		[]string{"--sample-interval", "15"},
	); len(d) != 1 ||
		!strings.Contains(d[0].Message, "?sample-interval=") {
		t.Errorf("a link still can't set the sample interval: %v", d)
	}
}
