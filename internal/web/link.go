package web

import (
	"strings"

	"github.com/iilei/canned-schedules/internal/diag"
)

// linkFlags are the flags a link may set with ?arg=: the ones that change the result and have no control of their
// own on the page. Each takes a value.
var linkFlags = map[string]bool{
	"require-base":               true,
	"base":                       true,
	"error-unless-alert-pattern": true,
	"set-config-at":              true,
	"set-config-value":           true,
}

// controlParams are the flags the page has a control for: a link sets those with the control's own parameter, so the
// control shows the value and the reader can change it.
var controlParams = map[string]string{
	"sample-interval":       "?sample-interval=",
	"preview-downsample":    "?downsample=",
	"no-preview-downsample": "?downsample=1",
	"ascii-symbols":         "?symbols=",
}

// checkLink reports every argument of a link (?arg=) the page can't take: flags it has a control for, flags that
// write files or pick the output, and words that aren't flags. Values are checked later, by the CLI's parser.
func checkLink(link []string) diag.List {
	var diags diag.List

	for i := 0; i < len(link); i++ {
		name, inline := flagName(link[i])
		if name == "" {
			diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
				"link argument %q is not a flag: every ?arg= is a flag or the value of the one before it",
				diag.Sanitize(link[i])))

			continue
		}

		if linkFlags[name] {
			if !inline {
				i++ // its value
			}

			continue
		}

		// A refused flag's value would otherwise be reported again as a word that isn't a flag.
		if !inline && i+1 < len(link) {
			if next, _ := flagName(link[i+1]); next == "" {
				i++
			}
		}

		switch {
		case controlParams[name] != "":
			diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
				"--%s can't come from ?arg=: the page has a control for it, linked with %s", name, controlParams[name]))
		default:
			diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
				"--%s can't come from a link: ?arg= takes only --require-base, --base, --error-unless-alert-pattern "+
					"and --set-config-at with --set-config-value", diag.Sanitize(name)))
		}
	}

	return diags
}

// flagName returns the name of a flag argument (-name, --name, --name=value) and whether its value is attached;
// "" for anything else.
func flagName(arg string) (string, bool) {
	rest, ok := strings.CutPrefix(arg, "-")
	if !ok {
		return "", false
	}

	rest = strings.TrimPrefix(rest, "-")
	name, _, inline := strings.Cut(rest, "=")

	if name == "" || strings.HasPrefix(name, "-") {
		return "", false
	}

	return name, inline
}
