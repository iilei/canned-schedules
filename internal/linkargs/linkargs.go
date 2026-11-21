// Package linkargs checks the flags that come from outside the page's controls: a link's ?arg=, a preset's args file
// and a self-hosted playground's site.json. They may set only the flags that change the result and have no control
// on the page.
package linkargs

import (
	"fmt"
	"strings"

	"github.com/iilei/canned-schedules/internal/diag"
)

// The config override flags, which come as a pair.
const (
	setConfigAt    = "set-config-at"
	setConfigValue = "set-config-value"
)

// linkFlags are the flags a link may set with ?arg=: the ones that change the result and have no control of their
// own on the page. Each takes a value.
var linkFlags = map[string]bool{
	"require-base":               true,
	"base":                       true,
	"error-unless-alert-pattern": true,
	setConfigAt:                  true,
	setConfigValue:               true,
}

// controlParams are the flags the page has a control for: a link sets those with the control's own parameter, so the
// control shows the value and the reader can change it.
var controlParams = map[string]string{
	"sample-interval":       "?sample-interval=",
	"preview-downsample":    "?downsample=",
	"no-preview-downsample": "?downsample=1",
	"ascii-symbols":         "?symbols=",
}

// siteFlags are the flags a self-hosted playground's site.json may set: a link's, plus the sample interval, which
// changes what is valid; the page shows the site's value in that control and locks it.
var siteFlags = map[string]bool{
	"require-base":               true,
	"base":                       true,
	"error-unless-alert-pattern": true,
	setConfigAt:                  true,
	setConfigValue:               true,
	"sample-interval":            true,
}

// occurrence is one flag with its value.
type occurrence struct{ name, value string }

// Check reports every argument of a link (?arg=) or a preset's args file that the page can't take: flags it has a
// control for, flags that write files or pick the output, and words that aren't flags. Values are checked later, by
// the CLI's parser.
func Check(link []string) diag.List {
	return check(link, linkFlags, func(name string) string {
		if controlParams[name] != "" {
			return fmt.Sprintf("--%s can't come from ?arg=: the page has a control for it, linked with %s", name,
				controlParams[name])
		}

		return fmt.Sprintf("--%s can't come from a link: ?arg= takes only --require-base, --base, "+
			"--error-unless-alert-pattern and --set-config-at with --set-config-value", diag.Sanitize(name))
	})
}

// CheckSite is Check for the args of a site.json, which may also set --sample-interval.
func CheckSite(args []string) diag.List {
	return check(args, siteFlags, func(name string) string {
		return fmt.Sprintf("--%s can't come from site.json: its args take only --require-base, --base, "+
			"--error-unless-alert-pattern, --sample-interval and --set-config-at with --set-config-value",
			diag.Sanitize(name))
	})
}

func check(args []string, allowed map[string]bool, refuse func(name string) string) diag.List {
	var diags diag.List

	for i := 0; i < len(args); i++ {
		name, inline := FlagName(args[i])
		if name == "" {
			diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
				"argument %q is not a flag: every argument is a flag or the value of the one before it",
				diag.Sanitize(args[i])))

			continue
		}

		if allowed[name] {
			if !inline {
				i++ // its value
			}

			continue
		}

		// A refused flag's value would otherwise be reported again as a word that isn't a flag.
		if !inline && i+1 < len(args) {
			if next, _ := FlagName(args[i+1]); next == "" {
				i++
			}
		}

		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag, "%s", refuse(name)))
	}

	return diags
}

// Conflicts reports every flag of a link or a preset that a site's flags already set: the same flag, or for
// --set-config-at the same or an overlapping config path (states and states[0], representation and
// representation.styles). The site's flags come last anyway, so they would win; reporting the conflict keeps a link
// from looking as if it loosened the check a team set up. Both lists passed their checks (Check, CheckSite), so every
// flag takes a value.
func Conflicts(site, link []string) diag.List {
	names := map[string]bool{}

	var paths []string

	for _, o := range occurrences(site) {
		switch o.name {
		case setConfigAt:
			paths = append(paths, o.value)
		case setConfigValue:
		default:
			names[o.name] = true
		}
	}

	var diags diag.List

	for _, o := range occurrences(link) {
		switch o.name {
		case setConfigValue:
		case setConfigAt:
			for _, p := range paths {
				if overlaps(o.value, p) {
					diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
						"--set-config-at %s: this playground's site.json sets %s, so it can't be changed here",
						diag.Sanitize(o.value), diag.Sanitize(p)))

					break
				}
			}
		default:
			if names[o.name] {
				diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
					"--%s is set by this playground's site.json, so it can't be changed here", diag.Sanitize(o.name)))
			}
		}
	}

	return diags
}

// occurrences lists the flags of args with their values (attached with = or the next argument).
func occurrences(args []string) []occurrence {
	var out []occurrence

	for i := 0; i < len(args); i++ {
		name, inline := FlagName(args[i])
		if name == "" {
			continue
		}

		o := occurrence{name: name}

		if inline {
			_, o.value, _ = strings.Cut(args[i], "=")
		} else if i+1 < len(args) {
			i++
			o.value = args[i]
		}

		out = append(out, o)
	}

	return out
}

// overlaps reports whether two config paths touch the same value: equal, or one inside the other.
func overlaps(a, b string) bool { return a == b || inside(a, b) || inside(b, a) }

func inside(p, parent string) bool {
	return strings.HasPrefix(p, parent+".") || strings.HasPrefix(p, parent+"[")
}

// FlagName returns the name of a flag argument (-name, --name, --name=value) and whether its value is attached;
// "" for anything else.
func FlagName(arg string) (string, bool) {
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
