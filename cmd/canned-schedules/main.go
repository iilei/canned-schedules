// Command canned-schedules validates a schedule document and prints its canonical JSON.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/iilei/canned-schedules/internal/cli"
)

// Set by goreleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "canned-schedules %s (%s, %s)\n", version, commit, date)

		return 0
	}

	opts, _ := cli.Parse(args)

	file := "-"
	if len(opts.Args) > 0 {
		file = opts.Args[0]
	}

	src, err := readInput(file, stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)

		return 1
	}

	out, diags, err := cli.Run(args, src)
	for _, d := range diags {
		fmt.Fprintln(stderr, d.Format(file))
	}

	if err != nil {
		fmt.Fprintln(stderr, err)

		return 1
	}

	if diags.HasErrors() {
		return 1
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")

	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(stderr, err)

		return 1
	}

	return 0
}

func readInput(file string, stdin io.Reader) ([]byte, error) {
	if file == "-" {
		return io.ReadAll(stdin)
	}

	return os.ReadFile(file) //nolint:gosec // Reading the file named on the command line is the point.
}
