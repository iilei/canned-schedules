package cli

import (
	"fmt"

	"github.com/iilei/canned-schedules/internal/render"
	"github.com/iilei/canned-schedules/internal/schedule"
)

// Trace explains the painting of a successfully evaluated document (--verbose, --trace-output-file).
func Trace(opts *Options, src []byte) (*schedule.Trace, error) {
	cfg, err := previewConfig(opts)
	if err != nil {
		return nil, err
	}

	tr, diags := schedule.Explain(src, cfg, opts.Schedule)
	if tr == nil {
		return nil, fmt.Errorf("trace: %v", diags)
	}

	return tr, nil
}

// Highlights computes the page's source highlights on the SVG of a successfully evaluated document: boxes around the
// name, summary and annotation rows, and the week in the preview's buckets (--preview-downsample). The JSON spans are
// left to the caller, which owns the JSON text.
func Highlights(opts *Options, out *schedule.Output, src []byte) ([]render.Highlight, error) {
	cfg, err := previewConfig(opts)
	if err != nil {
		return nil, err
	}

	tr, err := Trace(opts, src)
	if err != nil {
		return nil, err
	}

	p := render.Downsample(out, cfg, opts.Preview.Downsample)
	m := render.Build(out, cfg, p, svgOptions(opts, nil))

	return render.Highlights(tr, cfg, p, &m), nil
}
