// Command canned-schedules checks a weekly schedule request written in Markdown and prints its canonical JSON.
//
// Usage:
//
//	canned-schedules [flags] request.md
//
// The input is exactly one regular file of at most 256 KiB, given after the flags. On success, stdout carries the
// canonical JSON (or the ASCII preview with --output ascii), and further outputs (SVG and ASCII previews, MyST, the
// Sphinx kit) are written where flags ask for them. Problems go to stderr, or to --error-output-file, as an error
// report in txt, JSON or the built-in Markdown template.
//
// Exit codes: 0 when the document is valid, 1 when the document or a flag value has errors, 2 when the command line
// or the environment is unusable (no or several files, unreadable input, report not writable).
//
// canned-schedules --help lists every flag; the README documents the syntax, the outputs and the diagnostic codes.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/iilei/canned-schedules/internal/cli"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/internal/files"
	"github.com/iilei/canned-schedules/internal/report"
	"github.com/iilei/canned-schedules/internal/schedule"
)

// Exit codes: 1 means the document (or a flag value) has errors, 2 means the command line or the environment is
// unusable (no or several files, unreadable input, report not writable).
const (
	exitOK      = 0
	exitInvalid = 1
	exitUsage   = 2
)

const kib = 1 << 10

// maxInputBytes caps the input. GitHub issue bodies are limited to 65,536 characters, so this is generous.
const maxInputBytes = 256 * kib

// usageExample is the example attached to command-line usage errors.
const usageExample = "canned-schedules [flags] request.md"

// Set by goreleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, diags := cli.Parse(args)

	switch {
	case opts.Help:
		if err := cli.Help(stdout); err != nil {
			return exitInvalid
		}

		return exitOK
	case opts.Version && !diags.HasErrors():
		fmt.Fprintf(stdout, "canned-schedules %s (%s, %s)\n", version, commit, date)

		return exitOK
	}

	file, src, out, more := evaluate(&opts, diags.HasErrors())
	diags = append(diags, more...)

	if err := writeReport(opts.Report, report.Build(display(file), src, diags), stderr); err != nil {
		fmt.Fprintln(stderr, diag.Sanitize(err.Error()))

		return exitUsage
	}

	switch {
	case slices.ContainsFunc(diags, usageProblem):
		return exitUsage
	case diags.HasErrors():
		return exitInvalid
	}

	if err := writeOutputs(&opts, out, src); err != nil {
		fmt.Fprintln(stderr, diag.Sanitize(err.Error()))

		return exitUsage
	}

	if opts.Preview.Output == cli.OutputASCII {
		text, err := cli.ASCII(&opts, out, colorOn(opts.Preview.Color, stdout))
		if err != nil {
			fmt.Fprintln(stderr, err)

			return exitInvalid
		}

		if _, err := stdout.Write(text); err != nil {
			return exitInvalid
		}

		return exitOK
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false) // Excerpts start with ">"; this JSON is not embedded in HTML.

	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(stderr, err)

		return exitInvalid
	}

	return exitOK
}

// writeOutputs writes the requested output files: SVG and ASCII previews, MyST and the Sphinx kit. It only runs for
// documents without errors, so a failed run leaves earlier files untouched rather than replacing them with broken
// ones. The ASCII file gets colours only with --color always.
func writeOutputs(opts *cli.Options, out *schedule.Output, src []byte) error {
	for _, o := range []struct {
		render func() ([]byte, error)
		flag   string
		path   string
	}{
		{flag: "--svg-output-file", path: opts.Preview.SVGFile, render: func() ([]byte, error) {
			return cli.SVG(opts, out)
		}},
		{flag: "--ascii-output-file", path: opts.Preview.ASCIIFile, render: func() ([]byte, error) {
			return cli.ASCII(opts, out, opts.Preview.Color == cli.ColorAlways)
		}},
		{flag: "--myst-output-file", path: opts.Sphinx.MySTFile, render: func() ([]byte, error) {
			return cli.MyST(opts, src)
		}},
		{flag: "--sphinx-kit-output-file", path: opts.Sphinx.KitFile, render: func() ([]byte, error) {
			return cli.SphinxKit(opts, src, "canned-schedules "+version)
		}},
	} {
		if o.path == "" {
			continue
		}

		data, err := o.render()
		if err != nil {
			return fmt.Errorf("%s: %w", o.flag, err)
		}

		if err := files.WriteFile(o.path, display(o.path), data); err != nil {
			return fmt.Errorf("%s %w", o.flag, err)
		}
	}

	return nil
}

// colorOn decides colours for the ASCII preview on w: always, never, or (auto) only on a terminal and only when
// NO_COLOR is unset or empty (https://no-color.org).
func colorOn(mode string, w io.Writer) bool {
	switch mode {
	case cli.ColorAlways:
		return true
	case cli.ColorNever:
		return false
	}

	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	fi, err := f.Stat()

	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// evaluate finds and reads the input file and, unless the flags already failed, runs the pipeline on it. The file
// name is returned whenever it is known, so the report can name it.
func evaluate(opts *cli.Options, flagErrors bool) (string, []byte, *schedule.Output, diag.List) {
	file, argDiag := inputFile(opts.Args)
	if argDiag != nil {
		return "", nil, nil, diag.List{*argDiag}
	}

	if flagErrors {
		return file, nil, nil, nil
	}

	src, inDiag := readInput(file)
	if inDiag != nil {
		return file, nil, nil, diag.List{*inDiag}
	}

	out, diags := cli.Run(opts, src)

	return file, src, out, diags
}

// writeReport writes the report to --error-output-file (always, even without diagnostics, so a CI step never reads a
// stale report) or, when there is something to report, to stderr.
func writeReport(opts cli.ReportOptions, r report.Report, stderr io.Writer) error {
	if opts.File == "" && len(r.Diagnostics) == 0 {
		return nil
	}

	var buf bytes.Buffer
	if err := report.Render(&buf, r, opts.Format, opts.Template); err != nil {
		return err
	}

	if opts.File == "" {
		_, err := stderr.Write(buf.Bytes())

		return err
	}

	if err := files.WriteFile(opts.File, display(opts.File), buf.Bytes()); err != nil {
		return fmt.Errorf("--error-output-file %w", err)
	}

	return nil
}

// display shortens a path for messages and reports (home becomes "~"); file access always uses the path as given.
func display(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return files.DisplayPath(path, home)
}

func usageProblem(d diag.Diagnostic) bool {
	return d.Code == diag.CodeInvalidArgs || d.Code == diag.CodeInputFile
}

// inputFile returns the single positional argument. Flags must come before it: the flag package stops at the first
// non-flag argument, so anything after the file would otherwise be ignored silently.
func inputFile(positional []string) (string, *diag.Diagnostic) {
	var d diag.Diagnostic

	switch {
	case len(positional) == 1:
		return positional[0], nil
	case len(positional) == 0:
		d = diag.Errorf(diag.Pos{}, diag.CodeInvalidArgs, "missing input file")
	case strings.HasPrefix(positional[1], "-"):
		d = diag.Errorf(diag.Pos{}, diag.CodeInvalidArgs, "flags must come before the file, got %q after %q",
			positional[1], positional[0])
	default:
		d = diag.Errorf(diag.Pos{}, diag.CodeInvalidArgs, "expected one input file, got %d", len(positional))
	}

	d.Example = usageExample

	return "", &d
}

// readInput opens file once, checks the opened file (not the path again) and reads at most maxInputBytes.
func readInput(file string) ([]byte, *diag.Diagnostic) {
	fail := func(format string, args ...any) ([]byte, *diag.Diagnostic) {
		d := diag.Errorf(diag.Pos{}, diag.CodeInputFile, format, args...)

		return nil, &d
	}

	f, err := os.Open(file) //nolint:gosec // Reading the file named on the command line is the point.
	if err != nil {
		return fail("cannot open the input file: %s", files.Reason(err))
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fail("cannot read the input file: %s", files.Reason(err))
	}

	if !fi.Mode().IsRegular() {
		return fail("the input must be a regular file, not %s", describe(fi.Mode()))
	}

	src, err := io.ReadAll(io.LimitReader(f, maxInputBytes+1))
	if err != nil {
		return fail("cannot read the input file: %s", files.Reason(err))
	}

	if len(src) > maxInputBytes {
		return fail("the input file is larger than %d KiB", maxInputBytes/kib)
	}

	return src, nil
}

func describe(m os.FileMode) string {
	switch {
	case m.IsDir():
		return "a directory"
	case m&os.ModeNamedPipe != 0:
		return "a named pipe"
	case m&os.ModeSocket != 0:
		return "a socket"
	case m&os.ModeDevice != 0:
		return "a device"
	default:
		return "a special file"
	}
}
