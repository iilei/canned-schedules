package cannedschedules

import (
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/iilei/canned-schedules/internal/cli"
	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/render"
)

// modulePath identifies this module in the build info, for the default generator.
const modulePath = "github.com/iilei/canned-schedules"

// Themes. The zero value means ThemeAuto.
const (
	// ThemeAuto follows the viewer's prefers-color-scheme. Inline in a page with its own theme toggle, use a fixed
	// theme instead.
	ThemeAuto  Theme = render.ThemeAuto
	ThemeLight Theme = render.ThemeLight
	ThemeDark  Theme = render.ThemeDark
)

var errNoSource = errors.New("EmbedSource needs the document: use Result.SVG")

// Theme picks the SVG colours.
type Theme string

// SVGOptions configure an SVG preview, as the CLI's --svg-* flags do.
type SVGOptions struct {
	// Theme picks the colours (--svg-theme); empty means ThemeAuto.
	Theme Theme
	// Generator names the tool in the embedded source's generator attribute; empty means "canned-schedules"
	// followed by this module's version.
	Generator string
	// EmbedSource puts the schedule in canonical form into the SVG's metadata (--svg-embed-source). Off by default:
	// x-* values such as owners or links would travel with every shared picture.
	EmbedSource bool
	// Annotations shows the META section (summary and x-* keys) above the transition table (--svg-annotations).
	Annotations bool
}

// SVG is the week preview as the CLI's --svg-output-file writes it, with the buckets of Options.Preview.
func (r *Result) SVG(o SVGOptions) ([]byte, error) {
	if err := r.canPreview(); err != nil {
		return nil, err
	}

	opts := r.opts
	if err := svgOptions(&opts, o); err != nil {
		return nil, err
	}

	return cli.SVG(&opts, r.out, r.src, generatorOr(o.Generator))
}

// RenderSVG renders the week preview of a stored canonical output, e.g. a decoded .canned-schedule.json, without
// its document: for a re-render in another theme or a check that a published preview is current. opts must name
// the config overrides the output was made with, so its states and colours resolve; only opts.Config and
// opts.Preview are used (nil opts or Preview mean DefaultDownsample). The output must be valid against the output
// schema of SchemaVersion, and every state it names must be in the config. EmbedSource is not possible here.
func RenderSVG(out *Output, opts *Options, o SVGOptions) ([]byte, error) {
	if o.EmbedSource {
		return nil, fmt.Errorf("%w: %w", ErrInvalidOptions, errNoSource)
	}

	if opts == nil {
		opts = &Options{}
	}

	c, err := cliOptions(&Options{Config: opts.Config, Preview: opts.Preview})
	if err != nil {
		return nil, err
	}

	if err := svgOptions(&c, o); err != nil {
		return nil, err
	}

	cfg, diags := config.Load(c.Overrides)
	if cfg == nil {
		return nil, fmt.Errorf("%w: config: %s", ErrInvalidOptions, diagnostics(diags)[0])
	}

	in, err := toInternal(out)
	if err != nil {
		return nil, err
	}

	if err := in.Finish(); err != nil {
		return nil, fmt.Errorf("the output fails its schema: %w", err)
	}

	if err := knownStates(out, cfg); err != nil {
		return nil, err
	}

	return cli.SVG(&c, in, nil, "")
}

func svgOptions(c *cli.Options, o SVGOptions) error {
	switch o.Theme {
	case "":
		c.Preview.SVGTheme = render.ThemeAuto
	case ThemeAuto, ThemeLight, ThemeDark:
		c.Preview.SVGTheme = string(o.Theme)
	default:
		return fmt.Errorf("%w: Theme %q: use auto, light or dark", ErrInvalidOptions, o.Theme)
	}

	c.Preview.SVGEmbedSource, c.Preview.SVGAnnotations = o.EmbedSource, o.Annotations

	return nil
}

// knownStates checks that every state out names is configured, so the preview can colour it.
func knownStates(out *Output, cfg *config.Config) error {
	w := out.Week
	names := []string{out.Base}

	for _, day := range [][]Run{w.MON, w.TUE, w.WED, w.THU, w.FRI, w.SAT, w.SUN} {
		for _, run := range day {
			names = append(names, run.State)
		}
	}

	for _, t := range out.Transitions {
		names = append(names, t.State)
	}

	for _, n := range names {
		if _, ok := cfg.State(n); !ok {
			return fmt.Errorf("%w: the output names state %q, which the config doesn't have", ErrInvalidOptions, n)
		}
	}

	return nil
}

func generatorOr(g string) string {
	if g != "" {
		return g
	}

	return "canned-schedules " + moduleVersion()
}

// moduleVersion is this module's version in the running binary: the main module's when it is this one (the CLI,
// a checkout), otherwise the dependency's. "dev" when there is none.
func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	v := info.Main.Version
	if info.Main.Path != modulePath {
		v = ""

		for _, d := range info.Deps {
			if d.Path == modulePath {
				v = d.Version
			}
		}
	}

	if v = strings.TrimPrefix(v, "v"); v == "" || v == "(devel)" {
		return "dev"
	}

	return v
}
