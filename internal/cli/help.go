package cli

import (
	"fmt"
	"io"
	"strings"
)

// Layout of the flag list in --help.
const (
	helpWidth  = 100
	helpIndent = 2
	helpGap    = 2
)

// argPath is the placeholder of flags that take a file path.
const argPath = "<path>"

// flagGroups orders the flags for people and names their argument and default. The usage text comes from the flag
// set itself (newFlagSet); a test checks that every registered flag appears here exactly once.
var flagGroups = []struct {
	title string
	flags [][3]string // name, argument, default
}{
	{"Evaluation", [][3]string{
		{"sample-interval", "<minutes>", "`1`"},
		{"require-base", "true|false", "`false`"},
		{"base", "<state>", "first state"},
		{"error-unless-alert-pattern", "<regex>", ""},
	}},
	{"Configuration", [][3]string{
		{"set-config-at", argPath, ""},
		{"set-config-value", "<json>", ""},
		{"ascii-symbols", "<preset>", "`ascii`"},
	}},
	{"Outputs", [][3]string{
		{"output", "json|ascii|text", "`json`"},
		{"json-output-file", argPath, ""},
		{"svg-output-file", argPath, ""},
		{"svg-theme", "auto|light|dark", "`auto`"},
		{"svg-embed-source", "", ""},
		{"ascii-output-file", argPath, ""},
		{"text-output-file", argPath, ""},
		{"color", "auto|always|never", "`auto`"},
		{"preview-downsample", "[=N]", "`15`"},
		{"no-preview-downsample", "", ""},
		{"myst-output-file", argPath, ""},
		{"sphinx-kit-output-file", argPath, ""},
	}},
	{"Error report", [][3]string{
		{"error-output-file", argPath, "stderr"},
		{"error-output-format", "txt|json", "`txt`"},
		{"error-report-template", "<name>", ""},
	}},
	{"Other", [][3]string{
		{"help", "", ""},
		{"version", "", ""},
	}},
}

// FlagDoc documents one flag for --help and the README's flag table.
type FlagDoc struct {
	Group string
	// Name is the flag without dashes.
	Name string
	// Arg is the argument placeholder as shown, e.g. argPath or "json|ascii"; empty for switches. One starting with
	// "[" attaches to the name (--preview-downsample[=N]).
	Arg string
	// Default is the default as shown; empty when there is none worth showing.
	Default string
	// Usage is the flag's usage string from the flag set, with Markdown code spans.
	Usage string
}

// FlagDocs returns every flag in the order of flagGroups, with its usage string from the flag set.
func FlagDocs() []FlagDoc {
	fs := newFlagSet(&Options{}, &pending{})

	var docs []FlagDoc

	for _, g := range flagGroups {
		for _, f := range g.flags {
			usage := ""
			if fl := fs.Lookup(f[0]); fl != nil {
				usage = fl.Usage
			}

			docs = append(docs, FlagDoc{Group: g.title, Name: f[0], Arg: f[1], Default: f[2], Usage: usage})
		}
	}

	return docs
}

// Synopsis returns the flag as written on the command line: "--base <state>", "--preview-downsample[=N]".
func (d *FlagDoc) Synopsis() string {
	switch {
	case d.Arg == "":
		return "--" + d.Name
	case strings.HasPrefix(d.Arg, "["):
		return "--" + d.Name + d.Arg
	default:
		return "--" + d.Name + " " + d.Arg
	}
}

// Help writes the --help text: what the tool does, the usage line, the flags by group, and the exit codes.
func Help(w io.Writer) error {
	var b strings.Builder

	b.WriteString("canned-schedules checks a weekly schedule request written in Markdown\nand prints its canonical " +
		"JSON.\n\nUsage: canned-schedules [flags] request.md\n")

	docs := FlagDocs()

	width := 0
	for i := range docs {
		width = max(width, len(docs[i].Synopsis()))
	}

	group := ""

	for i := range docs {
		d := &docs[i]
		if d.Group != group {
			group = d.Group
			_, _ = fmt.Fprintf(&b, "\n%s:\n", group)
		}

		usage := plain(d.Usage)
		if d.Default != "" {
			usage += " (default: " + plain(d.Default) + ")"
		}

		// Descriptions wrap with a hanging indent so the help stays within helpWidth columns.
		indent := helpIndent + width + helpGap
		for i, line := range wrap(usage, helpWidth-indent) {
			if i == 0 {
				_, _ = fmt.Fprintf(&b, "%*s%-*s%*s%s\n", helpIndent, "", width, d.Synopsis(), helpGap, "", line)
			} else {
				_, _ = fmt.Fprintf(&b, "%*s%s\n", indent, "", line)
			}
		}
	}

	b.WriteString("\nExit codes: 0 valid, 1 errors in the document or a flag value,\n" +
		"2 unusable command line or environment.\nDocumentation: https://github.com/iilei/canned-schedules#readme\n")

	_, err := io.WriteString(w, b.String())

	return err
}

// wrap splits s into lines of at most width runes at spaces; a longer word stays on its own line.
func wrap(s string, width int) []string {
	var (
		lines []string
		line  string
	)

	for word := range strings.FieldsSeq(s) {
		switch {
		case line == "":
			line = word
		case len([]rune(line))+1+len([]rune(word)) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}

	return append(lines, line)
}

// plain drops Markdown code-span backticks for the terminal.
func plain(s string) string {
	return strings.ReplaceAll(s, "`", "")
}
