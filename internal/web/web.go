// Package web is the platform-independent core of the browser preflight page: one call evaluates a document and
// returns everything the page shows. cmd/canned-schedules-wasm exposes it to JavaScript.
package web

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/iilei/canned-schedules/internal/cli"
	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/htmlpreview"
	"github.com/iilei/canned-schedules/internal/report"
	"github.com/iilei/canned-schedules/internal/sphinxkit"
)

// fileName names the document in reports; the page has no file.
const fileName = "request.md"

// errInvalid means the document has errors, so there is nothing to put into a kit.
var errInvalid = errors.New("fix the errors first: the Sphinx kit needs a valid request")

// Result is what the page shows. Previews of the Markdown are rendered even when the document has errors; the
// canonical JSON, the schedule previews and the MyST only when it has none.
type Result struct {
	JSON           string `json:"json"`
	SVG            string `json:"svg"`
	ASCII          string `json:"ascii"`
	MyST           string `json:"myst"`
	Report         string `json:"report"`
	ReportMarkdown string `json:"report-markdown"`
	GitHubHTML     string `json:"github-html"`
	SphinxHTML     string `json:"sphinx-html"`
	SphinxCSS      string `json:"sphinx-css"`
	Errors         int    `json:"errors"`
	Warnings       int    `json:"warnings"`
}

// Evaluate runs the CLI pipeline on src with the given command-line flags (no file argument).
func Evaluate(src []byte, args []string) Result {
	var res Result

	opts, diags := cli.Parse(args)
	if !diags.HasErrors() {
		out, more := cli.Run(&opts, src)
		diags = append(diags, more...)

		if out != nil {
			res.JSON = encode(out)
			res.SVG = string(must(cli.SVG(&opts, out, src, "")))
			res.ASCII = string(must(cli.ASCII(&opts, out, false)))
			res.MyST = string(must(cli.MyST(&opts, src)))
		}
	}

	if cfg, _ := config.Load(opts.Overrides); cfg != nil {
		res.GitHubHTML, _ = htmlpreview.Render(src, cfg, htmlpreview.GitHub)
		res.SphinxHTML, _ = htmlpreview.Render(src, cfg, htmlpreview.Sphinx)
		res.SphinxCSS, _ = sphinxkit.CSS(cfg, sphinxkit.Page)
	}

	r := report.Build(fileName, src, diags)
	res.Report = encode(r)
	res.ReportMarkdown = string(must(render(r)))
	res.Errors, res.Warnings = r.Summary.Errors, r.Summary.Warnings

	return res
}

// Encode returns the result as a JSON string for JavaScript.
func (r *Result) Encode() string { return encode(r) }

func render(r report.Report) ([]byte, error) {
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

// SphinxKit builds the Sphinx kit zip for src with the given flags. The page calls it on demand, not on every edit.
func SphinxKit(src []byte, args []string) ([]byte, error) {
	opts, diags := cli.Parse(args)
	if diags.HasErrors() {
		return nil, errInvalid
	}

	if out, more := cli.Run(&opts, src); out == nil || more.HasErrors() {
		return nil, errInvalid
	}

	return cli.SphinxKit(&opts, src, "canned-schedules preflight page")
}
