package cannedschedules_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cs "github.com/iilei/canned-schedules"
	"github.com/iilei/canned-schedules/internal/cli"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/report"
)

const goldenDir = "testdata/golden"

const generator = "canned-schedules test"

const doc = "# Office Hours\n\n> [!SCHEDULE-STATE] Medium\n> - `08:00` to `18:00` MON-FRI\n"

// TestGoldenParity runs every golden fixture through the library and through the CLI's pipeline with the same
// options: diagnostics, the JSON, the report, the canonical form and every preview must be identical, and Output
// must encode to exactly the CLI's JSON (so the mirrored types lose nothing).
func TestGoldenParity(t *testing.T) {
	t.Parallel()

	for _, group := range []string{"valid", "warn", "error"} {
		cases, err := os.ReadDir(filepath.Join(goldenDir, group))
		if err != nil {
			t.Fatal(err)
		}

		for _, c := range cases {
			dir := filepath.Join(goldenDir, group, c.Name())
			t.Run(group+"/"+c.Name(), func(t *testing.T) {
				t.Parallel()
				parity(t, dir)
			})
		}
	}
}

func parity(t *testing.T, dir string) {
	t.Helper()

	src := read(t, filepath.Join(dir, "input.md"))
	args := lines(read(t, filepath.Join(dir, "args")))

	opts, flagDiags := cli.Parse(args)
	if flagDiags.HasErrors() {
		t.Skip("flag errors are the CLI's own; the library's options are checked in TestInvalidOptions")
	}

	lib := libOptions(&opts)

	r, err := cs.Evaluate([]byte(src), &lib)
	if err != nil {
		t.Fatal(err)
	}

	out, diags := cli.Run(&opts, []byte(src))

	var want []string
	for _, d := range diags {
		want = append(want, d.Format(""))
	}

	var got []string
	for _, d := range r.Diagnostics {
		got = append(got, d.String())
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("diagnostics:\n got %q\nwant %q", got, want)
	}

	compareReport(t, r, report.Build("input.md", []byte(src), diags))

	if out == nil {
		if r.Valid() || r.Output != nil {
			t.Fatal("the library has an output, the CLI none")
		}

		if _, err := r.JSON(); !errors.Is(err, cs.ErrInvalidDocument) {
			t.Errorf("JSON of an invalid document: %v", err)
		}

		return
	}

	wantJSON, err := cli.JSON(out)
	if err != nil {
		t.Fatal(err)
	}

	same(t, "JSON", wantJSON, must(r.JSON()))
	same(t, "Output encoded", wantJSON, encode(t, r.Output))
	same(t, "canonical", must(cli.Canonical(&opts, []byte(src))), must(r.Canonical()))

	if lib.Preview == nil {
		return
	}

	same(t, "ASCII", must(cli.ASCII(&opts, out, true)), must(r.ASCII(true)))
	same(t, "text form", must(cli.Text(&opts, out, []byte(src), false)), must(r.Text(false)))

	svgOpts := cs.SVGOptions{
		Theme: cs.Theme(opts.Preview.SVGTheme), EmbedSource: opts.Preview.SVGEmbedSource,
		Annotations: opts.Preview.SVGAnnotations, Generator: generator,
	}
	svg := must(r.SVG(svgOpts))
	same(t, "SVG", must(cli.SVG(&opts, out, []byte(src), generator)), svg)

	if !svgOpts.EmbedSource {
		same(t, "RenderSVG", svg, must(cs.RenderSVG(r.Output, &lib, svgOpts)))
	}
}

// libOptions translates parsed CLI flags into library options.
func libOptions(opts *cli.Options) cs.Options {
	lib := cs.Options{
		SampleInterval: opts.Schedule.SampleInterval, RequireBase: opts.Schedule.RequireBase,
		Base: opts.Schedule.Base,
	}

	if re := opts.Schedule.AlertPattern; re != nil {
		lib.AlertPattern = strings.TrimSuffix(strings.TrimPrefix(re.String(), "^(?:"), ")$")
	}

	for _, o := range opts.Overrides {
		lib.Config = append(lib.Config, cs.ConfigOverride{Path: o.Path, Value: o.Value})
	}

	if opts.Preview.Requested() {
		lib.Preview = &cs.PreviewOptions{Downsample: opts.Preview.Downsample}
	}

	return lib
}

func compareReport(t *testing.T, r *cs.Result, want report.Report) {
	t.Helper()

	p := r.Report("input.md")
	if p.Errors() != want.Summary.Errors || p.Warnings() != want.Summary.Warnings {
		t.Errorf("report counts: %d/%d, want %d/%d", p.Errors(), p.Warnings(), want.Summary.Errors,
			want.Summary.Warnings)
	}

	for _, f := range []struct {
		got            func() ([]byte, error)
		format, layout string
	}{
		{p.JSON, report.FormatJSON, ""},
		{p.Markdown, "", report.TemplateMarkdownGFM},
		{p.Text, report.FormatTXT, ""},
	} {
		var buf bytes.Buffer
		if err := report.Render(&buf, want, f.format, f.layout); err != nil {
			t.Fatal(err)
		}

		same(t, "report "+f.format+f.layout, buf.Bytes(), must(f.got()))
	}
}

func TestInvalidOptions(t *testing.T) {
	t.Parallel()

	for name, opts := range map[string]*cs.Options{
		"sample interval":      {SampleInterval: 7},
		"negative interval":    {SampleInterval: -5},
		"downsample":           {Preview: &cs.PreviewOptions{Downsample: 25}},
		"base and requirement": {Base: "Off", RequireBase: true},
		"alert pattern":        {AlertPattern: "NOTE|("},
	} {
		if _, err := cs.Evaluate([]byte(doc), opts); !errors.Is(err, cs.ErrInvalidOptions) {
			t.Errorf("%s: got %v, want ErrInvalidOptions", name, err)
		}
	}
}

func TestConfigProblemsAreDiagnostics(t *testing.T) {
	t.Parallel()

	r, err := cs.Evaluate([]byte(doc), &cs.Options{Config: []cs.ConfigOverride{{Path: "states", Value: "[]"}}})
	if err != nil {
		t.Fatal(err)
	}

	if r.Valid() || len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != diag.CodeConfigSchema {
		t.Errorf("want a config-schema diagnostic, got %v", r.Diagnostics)
	}
}

func TestPreviewNeedsOptions(t *testing.T) {
	t.Parallel()

	r, err := cs.Evaluate([]byte(doc), nil)
	if err != nil {
		t.Fatal(err)
	}

	for name, f := range map[string]func() ([]byte, error){
		"SVG":   func() ([]byte, error) { return r.SVG(cs.SVGOptions{}) },
		"ASCII": func() ([]byte, error) { return r.ASCII(false) },
		"Text":  func() ([]byte, error) { return r.Text(false) },
	} {
		if _, err := f(); !errors.Is(err, cs.ErrNoPreview) {
			t.Errorf("%s without Options.Preview: %v", name, err)
		}
	}

	if _, err := r.Canonical(); err != nil {
		t.Errorf("Canonical needs no preview: %v", err)
	}
}

func TestSVGOptions(t *testing.T) {
	t.Parallel()

	r, err := cs.Evaluate([]byte(doc), &cs.Options{Preview: &cs.PreviewOptions{}})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.SVG(cs.SVGOptions{Theme: "sepia"}); !errors.Is(err, cs.ErrInvalidOptions) {
		t.Errorf("unknown theme: %v", err)
	}

	svg := string(must(r.SVG(cs.SVGOptions{EmbedSource: true})))
	if !strings.Contains(svg, `generator="canned-schedules `) {
		t.Errorf("the default generator names the tool: %s", svg[:min(len(svg), 400)])
	}

	if _, err := cs.RenderSVG(r.Output, nil, cs.SVGOptions{EmbedSource: true}); !errors.Is(err,
		cs.ErrInvalidOptions) {
		t.Errorf("RenderSVG can't embed the source: %v", err)
	}
}

func TestRenderSVGChecksTheOutput(t *testing.T) {
	t.Parallel()

	r, err := cs.Evaluate([]byte(doc), nil)
	if err != nil {
		t.Fatal(err)
	}

	// The default downsample applies without Preview, so this is the SVG of a run with Preview set.
	want := must(must2(cs.Evaluate([]byte(doc), &cs.Options{Preview: &cs.PreviewOptions{}})).SVG(cs.SVGOptions{}))
	same(t, "RenderSVG", want, must(cs.RenderSVG(r.Output, nil, cs.SVGOptions{})))

	renamed := []cs.ConfigOverride{{Path: "states", Value: `[{"id": "Closed", "display-name": "Closed"}]`}}
	if _, err := cs.RenderSVG(r.Output, &cs.Options{Config: renamed}, cs.SVGOptions{}); !errors.Is(err,
		cs.ErrInvalidOptions) {
		t.Errorf("states missing from the config: %v", err)
	}

	broken := *r.Output
	broken.SchemaVersion = "1"

	if _, err := cs.RenderSVG(&broken, nil, cs.SVGOptions{}); err == nil {
		t.Error("an output that fails its schema must be refused")
	}
}

func read(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	} else if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

// lines splits an args file: one argument per line, empty lines skipped.
func lines(s string) []string {
	var out []string

	for line := range strings.SplitSeq(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}

	return out
}

func encode(t *testing.T, v any) []byte {
	t.Helper()

	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func same(t *testing.T, what string, want, got []byte) {
	t.Helper()

	if !bytes.Equal(want, got) {
		t.Errorf("%s differs from the CLI's:\n got %.300q\nwant %.300q", what, got, want)
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}

	return b
}

func must2(r *cs.Result, err error) *cs.Result {
	if err != nil {
		panic(err)
	}

	return r
}
