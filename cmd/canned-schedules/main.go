// Command canned-schedules validates a schedule document and prints its canonical JSON.
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
	"github.com/iilei/canned-schedules/internal/report"
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

const usage = "usage: canned-schedules [flags] <file>"

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
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "canned-schedules %s (%s, %s)\n", version, commit, date)

		return exitOK
	}

	opts, diags := cli.Parse(args)
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

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")

	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(stderr, err)

		return exitInvalid
	}

	return exitOK
}

// evaluate finds and reads the input file and, unless the flags already failed, runs the pipeline on it. The file
// name is returned whenever it is known, so the report can name it.
func evaluate(opts *cli.Options, flagErrors bool) (string, []byte, any, diag.List) {
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
	if out == nil {
		return file, src, nil, diags
	}

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

	return report.WriteFile(opts.File, display(opts.File), buf.Bytes())
}

// display shortens a path for messages and reports (home becomes "~"); file access always uses the path as given.
func display(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return report.DisplayPath(path, home)
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
		d = diag.Errorf(diag.Pos{}, diag.CodeInvalidArgs, "missing input file; %s", usage)
	case strings.HasPrefix(positional[1], "-"):
		d = diag.Errorf(diag.Pos{}, diag.CodeInvalidArgs, "flags must come before the file, got %q after %q; %s",
			positional[1], positional[0], usage)
	default:
		d = diag.Errorf(diag.Pos{}, diag.CodeInvalidArgs, "expected one input file, got %d; %s", len(positional),
			usage)
	}

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
		return fail("cannot open the input file: %s", report.Reason(err))
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fail("cannot read the input file: %s", report.Reason(err))
	}

	if !fi.Mode().IsRegular() {
		return fail("the input must be a regular file, not %s", describe(fi.Mode()))
	}

	src, err := io.ReadAll(io.LimitReader(f, maxInputBytes+1))
	if err != nil {
		return fail("cannot read the input file: %s", report.Reason(err))
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
