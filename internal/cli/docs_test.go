package cli

import (
	"flag"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/iilei/canned-schedules/internal/diag"
)

const readme = "../../README.md"

// TestFlagDocsCoverFlagSet keeps --help and the README complete: every registered flag appears exactly once in
// flagGroups, with a usage string.
func TestFlagDocsCoverFlagSet(t *testing.T) {
	t.Parallel()

	documented := map[string]int{}
	for _, d := range FlagDocs() {
		documented[d.Name]++

		if d.Usage == "" {
			t.Errorf("--%s is listed in flagGroups but not registered", d.Name)
		}
	}

	newFlagSet(&Options{}, &pending{}).VisitAll(func(f *flag.Flag) {
		if documented[f.Name] != 1 {
			t.Errorf("--%s appears %d times in flagGroups, want once", f.Name, documented[f.Name])
		}
	})
}

func TestHelp(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	if err := Help(&b); err != nil {
		t.Fatal(err)
	}

	for line := range strings.SplitSeq(b.String(), "\n") {
		if n := len([]rune(line)); n > helpWidth {
			t.Errorf("help line is %d columns, want at most %d: %q", n, helpWidth, line)
		}

		if strings.Contains(line, "`") {
			t.Errorf("help line keeps Markdown backticks: %q", line)
		}
	}

	for _, want := range []string{"Usage: canned-schedules [flags] request.md", "--preview-downsample[=N]", "Exit codes:"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("help is missing %q", want)
		}
	}
}

func TestHelpFlags(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"-h"}, {"--help"}, {"-help"}, {"--help", "request.md"}} {
		opts, diags := Parse(args)
		if !opts.Help || len(diags) != 0 {
			t.Errorf("%q: Help = %v, diagnostics %v", args, opts.Help, diags)
		}
	}
}

// TestReadmeReference keeps the README's generated sections (the flag tables and the diagnostic codes) in sync with
// the code. Regenerate them with: mise run docs (UPDATE_DOCS=1 go test ./internal/cli -run TestReadmeReference).
func TestReadmeReference(t *testing.T) {
	t.Parallel()

	current := readFile(t, readme)
	want := current

	for _, s := range []struct{ name, body string }{
		{"flags", flagTables()},
		{"codes", codeTable()},
	} {
		section := regexp.MustCompile(`(?s)(<!-- generated: ` + s.name + ` [^\n]*-->\n).*?(<!-- end generated -->)`)
		if !section.MatchString(want) {
			t.Fatalf("the README has no generated %q section", s.name)
		}

		want = section.ReplaceAllStringFunc(want, func(m string) string {
			sub := section.FindStringSubmatch(m)

			return sub[1] + s.body + sub[2]
		})
	}

	if want == current {
		return
	}

	if os.Getenv("UPDATE_DOCS") == "" {
		t.Fatal("the README's generated sections are stale; run: mise run docs")
	}

	if err := os.WriteFile(readme, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
}

// flagTables renders the flags as one table per group, each under a bold group title.
func flagTables() string {
	var (
		b     strings.Builder
		group string
		rows  [][]string
	)

	flush := func() {
		if rows != nil {
			b.WriteString("**" + group + "**\n\n" + table([]string{"Flag", "Default", "Meaning"}, rows) + "\n")
		}
	}

	docs := FlagDocs()
	for i := range docs {
		d := &docs[i]
		if d.Group != group {
			flush()
			group, rows = d.Group, nil
		}

		rows = append(rows, []string{"`" + strings.ReplaceAll(d.Synopsis(), "|", `\|`) + "`", d.Default, d.Usage})
	}

	flush()

	return strings.TrimSuffix(b.String(), "\n")
}

// codeTable renders every diagnostic code with its severity and meaning.
func codeTable() string {
	rows := make([][]string, 0, len(diag.Codes))

	for _, c := range diag.Codes {
		severity := "error"
		if c.Severity == diag.Warn {
			severity = "warning"
		}

		rows = append(rows, []string{"`" + c.Code + "`", severity, c.Meaning})
	}

	return table([]string{"Code", "Severity", "Meaning"}, rows)
}

// table renders a Markdown table with padded columns, like the hand-written tables in the README.
func table(header []string, rows [][]string) string {
	width := make([]int, len(header))
	for _, r := range append([][]string{header}, rows...) {
		for i, c := range r {
			width[i] = max(width[i], len([]rune(c)))
		}
	}

	line := func(cells []string) string {
		padded := make([]string, len(cells))
		for i, c := range cells {
			padded[i] = c + strings.Repeat(" ", width[i]-len([]rune(c)))
		}

		return "| " + strings.Join(padded, " | ") + " |\n"
	}

	dashes := make([]string, len(header))
	for i := range header {
		dashes[i] = strings.Repeat("-", width[i])
	}

	var b strings.Builder

	b.WriteString(line(header))
	b.WriteString(line(dashes))

	for _, r := range rows {
		b.WriteString(line(r))
	}

	return b.String()
}
