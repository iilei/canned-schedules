package cli

import (
	"flag"
	"slices"
	"strconv"
	"strings"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/render"
)

// Preview flag values.
const (
	OutputJSON  = "json"
	OutputASCII = "ascii"
	OutputText  = "text"

	ColorAuto   = "auto"
	ColorAlways = "always"
	ColorNever  = "never"

	asciiSymbolsPath = "representation.ascii.symbols"

	// DefaultDownsample is the preview bucket in minutes without --preview-downsample=N: 96 columns per day.
	DefaultDownsample = 15
)

// PreviewOptions configure the visual outputs (SVG and ASCII). They never change the canonical JSON content, only
// whether a preview-approximate warning is added.
type PreviewOptions struct {
	// Output is --output: json (default) prints the canonical JSON to stdout, ascii the ASCII preview, text the text
	// form (the canonical request with the ASCII preview).
	Output string
	// JSONFile is --json-output-file; empty means none.
	JSONFile string
	// TextFile is --text-output-file; empty means none.
	TextFile string
	// ASCIIFile is --ascii-output-file; empty means none.
	ASCIIFile string
	// Color is --color: auto (default), always or never.
	Color string
	// SVGFile is --svg-output-file; empty means none.
	SVGFile string
	// SVGTheme is --svg-theme: auto (follow the viewer), light or dark.
	SVGTheme string
	// downsampleRaw is the value of an explicit --preview-downsample ("true" when bare).
	downsampleRaw string
	// Downsample is the bucket size in minutes (--preview-downsample[=N], --no-preview-downsample means 1).
	Downsample int
	// SVGEmbedSource is --svg-embed-source: the SVG carries the canonical request in its metadata.
	SVGEmbedSource bool
	// noDownsample records --no-preview-downsample to detect the conflicting combination.
	noDownsample bool
	// downsampleSet records an explicit --preview-downsample.
	downsampleSet bool
}

// downsampleFlag implements --preview-downsample and --preview-downsample=N. As a boolean-style flag it never takes
// the next argument, so the value must be attached with "=".
type downsampleFlag struct{ p *PreviewOptions }

func (f downsampleFlag) String() string { return "" }

// IsBoolFlag lets the flag appear without a value.
func (downsampleFlag) IsBoolFlag() bool { return true }

// Requested reports whether any preview is produced. The text form contains the ASCII preview.
func (p *PreviewOptions) Requested() bool {
	return p.SVGFile != "" || p.ASCIIFile != "" || p.TextFile != "" || p.Output == OutputASCII ||
		p.Output == OutputText
}

// Set records the raw value; checkPreviewFlags validates it, because the flag package would word any error here as
// an "invalid boolean value".
func (f downsampleFlag) Set(s string) error {
	f.p.downsampleSet = true
	f.p.downsampleRaw = s

	return nil
}

func registerPreviewFlags(fs *flag.FlagSet, p *PreviewOptions) {
	p.Downsample = DefaultDownsample
	fs.StringVar(&p.Output, "output", OutputJSON, "What stdout carries: the canonical JSON, the ASCII preview or the "+
		"text form (the canonical request with the ASCII preview)")
	fs.StringVar(&p.JSONFile, "json-output-file", "",
		"Write the canonical JSON (only when the document has no errors)")
	fs.StringVar(&p.SVGFile, "svg-output-file", "", "Write an SVG preview (only when the document has no errors)")
	fs.StringVar(&p.SVGTheme, "svg-theme", render.ThemeAuto, "SVG colours: `auto` (follow the viewer), `light` or "+
		"`dark`")
	fs.BoolVar(&p.SVGEmbedSource, "svg-embed-source", false, "Embed the canonical request in the SVG's metadata, "+
		"with the schedule digest; needs `--svg-output-file`")
	fs.StringVar(&p.ASCIIFile, "ascii-output-file", "", "Write an ASCII preview (only when the document has no errors)")
	fs.StringVar(&p.TextFile, "text-output-file", "", "Write the text form: the canonical request with the ASCII "+
		"preview, itself a valid request (only when the document has no errors)")
	fs.StringVar(&p.Color, "color", ColorAuto, "Colours in the ASCII preview; `auto` means on a terminal only, and "+
		"not with `NO_COLOR`")
	fs.Var(downsampleFlag{p}, "preview-downsample", "Preview buckets in minutes (must divide 60); `=N` needs the `=`")
	fs.BoolVar(&p.noDownsample, "no-preview-downsample", false, "Preview at 1-minute resolution (1440 columns per day)")
}

func checkPreviewFlags(p *PreviewOptions) diag.List {
	var diags diag.List

	for _, c := range []struct {
		flag, value string
		allowed     []string
	}{
		{"--output", p.Output, []string{OutputJSON, OutputASCII, OutputText}},
		{"--color", p.Color, []string{ColorAuto, ColorAlways, ColorNever}},
		{"--svg-theme", p.SVGTheme, []string{render.ThemeAuto, render.ThemeLight, render.ThemeDark}},
	} {
		if !slices.Contains(c.allowed, c.value) {
			diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag, "%s %q: use %s", c.flag, c.value,
				strings.Join(c.allowed, ", ")))
		}
	}

	if p.SVGEmbedSource && p.SVGFile == "" {
		diags = append(diags, diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"--svg-embed-source needs --svg-output-file"))
	}

	return append(diags, checkDownsample(p)...)
}

func checkDownsample(p *PreviewOptions) diag.List {
	bad := func(format string, args ...any) diag.List {
		return diag.List{diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag, format, args...)}
	}

	if p.downsampleSet && p.downsampleRaw != "true" {
		n, err := strconv.Atoi(p.downsampleRaw)
		if err != nil {
			d := bad("--preview-downsample=%s: give the bucket size in minutes", p.downsampleRaw)
			d[0].Example = "--preview-downsample=30"

			return d
		}

		p.Downsample = n
	}

	switch {
	case p.noDownsample && p.downsampleSet:
		return bad("--preview-downsample and --no-preview-downsample can't be combined")
	case p.noDownsample:
		p.Downsample = 1
	case p.Downsample < 1 || minutesPerHour%p.Downsample != 0:
		return bad("--preview-downsample=%d: must divide 60 (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60)", p.Downsample)
	}

	return nil
}

// asciiSymbols turns --ascii-symbols into an override of representation.ascii.symbols. It comes after any
// --set-config-at pairs, so the dedicated flag wins.
func asciiSymbols(opts *Options, preset string) diag.List {
	switch preset {
	case "":
		return nil
	case config.SymbolsASCII, config.SymbolsBraille, config.SymbolsStyles:
		opts.Overrides = append(opts.Overrides, config.Override{Path: asciiSymbolsPath, Value: strconv.Quote(preset)})

		return nil
	default:
		return diag.List{diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag,
			"--ascii-symbols %q: use ascii, braille or styles", preset)}
	}
}
