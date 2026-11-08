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

// Highlights computes the page's source highlights on the SVG week of a successfully evaluated document, in the
// preview's buckets (--preview-downsample).
func Highlights(opts *Options, out *schedule.Output, src []byte) ([]render.Highlight, error) {
	cfg, err := previewConfig(opts)
	if err != nil {
		return nil, err
	}

	tr, err := Trace(opts, src)
	if err != nil {
		return nil, err
	}

	return render.Highlights(tr, cfg, render.Downsample(out, cfg, opts.Preview.Downsample)), nil
}
