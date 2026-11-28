# Go library

Tools written in Go can import the pipeline instead of running the binary. The package
`github.com/iilei/canned-schedules` evaluates a document and renders the same outputs the CLI writes: the JSON,
diagnostics with positions, the error report, the canonical Markdown, the text form, the ASCII and SVG previews,
and the SVG of a stored `.canned-schedule.json`. It never touches the network or the file system.

```go
r, err := cannedschedules.Evaluate(src, &cannedschedules.Options{
	// Plan previews, as the CLI's preview flags do.
	Preview: &cannedschedules.PreviewOptions{},
})
if err != nil {
	return err // unusable options only; document problems are r.Diagnostics
}
if !r.Valid() {
	md, _ := r.Report("schedule.md").Markdown()
	// post md
}
svg, err := r.SVG(cannedschedules.SVGOptions{Theme: cannedschedules.ThemeAuto})
```

The package documentation lists everything (`go doc github.com/iilei/canned-schedules`). Its exported API follows
the module's version, see [Stability](stability.md).
