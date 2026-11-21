// Package web is the platform-independent core of the browser preflight page: one call evaluates a document and
// returns everything the page shows. cmd/canned-schedules-wasm exposes it to JavaScript.
package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/iilei/canned-schedules/internal/cli"
	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/htmlpreview"
	"github.com/iilei/canned-schedules/internal/linkargs"
	"github.com/iilei/canned-schedules/internal/render"
	"github.com/iilei/canned-schedules/internal/report"
	"github.com/iilei/canned-schedules/internal/schedule"
	"github.com/iilei/canned-schedules/internal/sphinxkit"
)

// fileName names the document in reports; the page has no file.
const fileName = "schedule.md"

// The names the page gives itself as a generator: the public instance, or a team's copy with a site.json.
const (
	PreflightPage = "canned-schedules preflight page"
	Playground    = "canned-schedules playground"
)

// errInvalid means the document has errors, so there is nothing to put into a kit.
var errInvalid = errors.New("fix the errors first: the Sphinx kit needs a valid schedule")

// Result is what the page shows. Previews of the Markdown are rendered even when the document has errors; the
// canonical JSON, the schedule previews, the highlights, the MyST and the text form only when it has none. States are the
// effective states after the config overrides (the page's states editor starts from them), absent when the config
// doesn't load.
type Result struct {
	JSON           string `json:"json"`
	SVG            string `json:"svg"`
	ASCII          string `json:"ascii"`
	MyST           string `json:"myst"`
	Text           string `json:"text"`
	Report         string `json:"report"`
	ReportMarkdown string `json:"report-markdown"`
	GitHubHTML     string `json:"github-html"`
	SphinxHTML     string `json:"sphinx-html"`
	SphinxCSS      string `json:"sphinx-css"`
	// File is the catalog file name derived from the schedule's name (schedule.FileName); absent when the document
	// has errors. A self-hosted playground proposes the schedule under it.
	File   string         `json:"file,omitempty"`
	States []config.State `json:"states,omitempty"`
	// Highlights are the SVG areas each source entry paints, in the preview's buckets; the page stripes the ones
	// under the editor cursor. Absent when the document has errors.
	Highlights []render.Highlight `json:"highlights,omitempty"`
	Errors     int                `json:"errors"`
	Warnings   int                `json:"warnings"`
}

// Evaluate runs the CLI pipeline on src with the flags from a link (?arg=, checked by linkargs.Check) followed by the
// page's own flags (no file argument). The page's flags come last, so its controls win over a link's config
// overrides of the same path. A link with a flag the page can't take is reported and left out entirely.
func Evaluate(src []byte, link, args []string) Result {
	return EvaluateSite(src, nil, link, args, false)
}

// EvaluateSite is Evaluate on a self-hosted playground: site holds its site.json's flags, which come last, so neither
// a link, a preset nor the page's controls can loosen the check the team set up; a link or preset flag the site sets
// is reported (linkargs.Conflicts) and the link's flags are left out. playground says the page has a site.json (even one
// with errors), so the SVG names the playground as its generator instead of the preflight page.
func EvaluateSite(src []byte, site, link, args []string, playground bool) Result {
	var res Result

	opts, diags := parse(site, link, args)
	if !diags.HasErrors() {
		out, more := cli.Run(&opts, src)
		diags = append(diags, more...)

		if out != nil {
			// The page writes no file, so --svg-embed-source can't be passed; its SVG (shown and downloaded) always
			// carries the schedule in canonical form, so a downloaded picture stays traceable to its schedule.
			opts.Preview.SVGEmbedSource = true
			// The page shows the annotations section too: the editor already holds them, and its SVG carries the
			// source anyway.
			opts.Preview.SVGAnnotations = true
			res.JSON = encode(out)
			res.File = schedule.FileName(out.Name)
			res.SVG = string(must(cli.SVG(&opts, out, src, generator(playground))))
			res.ASCII = string(must(cli.ASCII(&opts, out, false)))
			res.MyST = string(must(cli.MyST(&opts, src)))
			res.Text = string(must(cli.Text(&opts, out, src, false)))
			res.Highlights, _ = cli.Highlights(&opts, out, src)
			if tr, err := cli.Trace(&opts, src); err == nil {
				attachJSON(res.Highlights, out, tr, res.JSON)
			}
		}
	}

	if cfg, _ := config.Load(opts.Overrides); cfg != nil {
		res.GitHubHTML, _ = htmlpreview.Render(src, cfg, htmlpreview.GitHub)
		res.SphinxHTML, _ = htmlpreview.Render(src, cfg, htmlpreview.Sphinx)
		res.SphinxCSS, _ = sphinxkit.CSS(cfg, sphinxkit.Page)
		res.States = cfg.States
	}

	r := report.Build(fileName, src, diags)
	res.Report = encode(r)
	res.ReportMarkdown = string(must(renderReport(r)))
	res.Errors, res.Warnings = r.Summary.Errors, r.Summary.Warnings

	return res
}

// parse checks the link's flags (a preset's come with them) and parses them with the page's and the site's. When the
// link has flags the page can't take or that the site sets, only the page's and the site's flags are parsed, so the
// report doesn't repeat them as unknown or misplaced. The site's flags come last: later flags win.
func parse(site, link, args []string) (cli.Options, diag.List) {
	diags := linkargs.CheckSite(site)
	if diags.HasErrors() {
		site = nil // The page checked site.json before passing it; this is a safety net.
	}

	more := linkargs.Check(link)
	if !more.HasErrors() {
		more = linkargs.Conflicts(site, link)
	}

	if diags = append(diags, more...); more.HasErrors() {
		link = nil
	}

	opts, parsed := cli.Parse(slices.Concat(link, args, site))

	return opts, append(diags, parsed...)
}

// Encode returns the result as a JSON string for JavaScript.
func (r *Result) Encode() string { return encode(r) }

func renderReport(r report.Report) ([]byte, error) {
	var buf bytes.Buffer
	err := report.Render(&buf, r, "", report.TemplateMarkdownGFM)

	return buf.Bytes(), err
}

func encode(v any) string {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)

	return buf.String()
}

// must drops the error of a rendering step that only fails on a bug; the page shows what it has.
func must(b []byte, _ error) []byte { return b }

// pageName is Playground on a team's copy, PreflightPage otherwise.
func pageName(playground bool) string {
	if playground {
		return Playground
	}

	return PreflightPage
}

// generator names the page and, when the build has one, the module version (WebAssembly builds from a checkout get
// it from git), for the SVG's source element.
func generator(playground bool) string {
	page := pageName(playground)

	if info, ok := debug.ReadBuildInfo(); ok {
		if v := strings.TrimPrefix(info.Main.Version, "v"); v != "" && v != "(devel)" {
			return page + " " + v
		}
	}

	return page
}

// SphinxKit builds the Sphinx kit zip for src with the given flags. The page calls it on demand, not on every edit.
func SphinxKit(src []byte, link, args []string) ([]byte, error) {
	return SphinxKitSite(src, nil, link, args, false)
}

// SphinxKitSite is SphinxKit with a self-hosted playground's flags and page name, as in EvaluateSite.
func SphinxKitSite(src []byte, site, link, args []string, playground bool) ([]byte, error) {
	opts, diags := parse(site, link, args)
	if diags.HasErrors() {
		return nil, errInvalid
	}

	if out, more := cli.Run(&opts, src); out == nil || more.HasErrors() {
		return nil, errInvalid
	}

	cfg, err := cli.Config(&opts)
	if err != nil {
		return nil, err
	}

	return sphinxkit.Kit(src, cfg, pageName(playground))
}
