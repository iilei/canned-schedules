// Command canned-schedules validates a schedule document and prints its canonical JSON.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/iilei/canned-schedules/internal/cli"
)

// Exit codes: 1 means the document (or a flag value) has errors, 2 means the command line itself is unusable.
const (
	exitOK      = 0
	exitInvalid = 1
	exitUsage   = 2
)

const usage = "usage: canned-schedules [flags] <file>"

var errMissingFile = errors.New("missing input file")

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

	opts, _ := cli.Parse(args)

	file, err := inputFile(opts.Args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n%s\n", err, usage)

		return exitUsage
	}

	src, err := os.ReadFile(file) //nolint:gosec // Reading the file named on the command line is the point.
	if err != nil {
		fmt.Fprintln(stderr, err)

		return exitUsage
	}

	out, diags := cli.Run(args, src)
	for _, d := range diags {
		fmt.Fprintln(stderr, d.Format(file))
	}

	if diags.HasErrors() {
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

// inputFile returns the single positional argument. Flags must come before it: the flag package stops at the first
// non-flag argument, so anything after the file would otherwise be ignored silently.
func inputFile(positional []string) (string, error) {
	switch {
	case len(positional) == 0:
		return "", errMissingFile
	case len(positional) > 1 && strings.HasPrefix(positional[1], "-"):
		return "", fmt.Errorf("flags must come before the file, got %q after %q", positional[1], positional[0])
	case len(positional) > 1:
		return "", fmt.Errorf("expected one input file, got %d: %s", len(positional), strings.Join(positional, " "))
	default:
		return positional[0], nil
	}
}
