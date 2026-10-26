package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const basic = "../../testdata/golden/valid/basic/input.md"

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wantStderr string
		args       []string
		wantCode   int
	}{
		{name: "valid file", args: []string{basic}, wantCode: exitOK},
		{name: "flags then file", args: []string{"--sample-interval", "60", basic}, wantCode: exitOK},
		{name: "no file", args: nil, wantCode: exitUsage, wantStderr: "missing input file"},
		// The code, not the OS's wording: "no such file" on Unix, "cannot find the file" on Windows.
		{name: "stdin dash is not a file", args: []string{"-"}, wantCode: exitUsage, wantStderr: "error[input-file]"},
		{name: "two files", args: []string{basic, basic}, wantCode: exitUsage, wantStderr: "expected one input file"},
		{
			name:       "flag after file",
			args:       []string{basic, "--sample-interval", "60"},
			wantCode:   exitUsage,
			wantStderr: "flags must come before the file",
		},
		{name: "missing file", args: []string{"nope.md"}, wantCode: exitUsage, wantStderr: "nope.md"},
		{
			name:       "document error reports file:line:col",
			args:       []string{"../../testdata/golden/error/time-24-00/input.md"},
			wantCode:   exitInvalid,
			wantStderr: "../../testdata/golden/error/time-24-00/input.md:9:17: error[invalid-time]",
		},
		{name: "invalid flag value", args: []string{"--sample-interval", "7", basic}, wantCode: exitInvalid},
		{name: "directory as input", args: []string{"."}, wantCode: exitUsage, wantStderr: "a directory"},
		{
			name:       "json report on stderr",
			args:       []string{"--error-output-format", "json", "../../testdata/golden/error/time-24-00/input.md"},
			wantCode:   exitInvalid,
			wantStderr: `"location": "../../testdata/golden/error/time-24-00/input.md:9:17"`,
		},
		{
			name: "markdown report on stderr",
			args: []string{
				"--error-report-template",
				"markdown-gfm",
				"../../testdata/golden/error/time-24-00/input.md",
			},
			wantCode:   exitInvalid,
			wantStderr: "#### error `invalid-time`",
		},
		{
			name:       "format and template are exclusive",
			args:       []string{"--error-output-format", "json", "--error-report-template", "markdown-gfm", basic},
			wantCode:   exitInvalid,
			wantStderr: "can't be combined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr.String())
			}

			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}

			if (code == exitOK) != (stdout.Len() > 0) {
				t.Errorf("stdout must hold the JSON exactly when the run succeeds, got %d bytes", stdout.Len())
			}
		})
	}
}

func TestHelpAndVersion(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		want string
		args []string
	}{
		{"Usage: canned-schedules [flags] request.md", []string{"--help"}},
		{"Usage: canned-schedules [flags] request.md", []string{"-h"}},
		{"Usage: canned-schedules [flags] request.md", []string{"--help", basic}},
		{"canned-schedules dev", []string{"--version"}},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tc.args, &stdout, &stderr); code != exitOK || !strings.Contains(stdout.String(), tc.want) ||
			stderr.Len() != 0 {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", tc.args, code, stdout.String(), stderr.String())
		}
	}
}

func TestReportFile(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(target, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	code := run([]string{"--error-output-format", "json", "--error-output-file", target, basic}, &stdout, &stderr)
	if code != exitOK || stderr.Len() != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}

	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(b), `"diagnostics": []`) {
		t.Errorf("a successful run must still write an (empty) report, got %s", b)
	}

	code = run([]string{"--error-output-file", t.TempDir(), basic}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "is a directory") {
		t.Errorf("a directory as report target: exit code = %d, stderr = %q", code, stderr.String())
	}
}

func TestSVGFile(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "preview.svg")

	var stdout, stderr bytes.Buffer

	if code := run([]string{"--svg-output-file", target, basic}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}

	b, err := os.ReadFile(target)
	if err != nil || !bytes.HasPrefix(b, []byte("<svg ")) {
		t.Fatalf("preview = %q, err = %v", b, err)
	}

	if err := os.WriteFile(target, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}

	code := run(
		[]string{"--svg-output-file", target, "../../testdata/golden/error/time-24-00/input.md"},
		&stdout,
		&stderr,
	)
	if code != exitInvalid {
		t.Fatalf("exit code = %d", code)
	}

	if b, _ := os.ReadFile(target); string(b) != "previous" {
		t.Error("a document with errors must not replace the preview")
	}
}

func TestASCIIOutput(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	if code := run([]string{"--output", "ascii", basic}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "MON :::") || strings.Contains(stdout.String(), `"schema-version"`) {
		t.Errorf("--output ascii must print the preview instead of the JSON:\n%s", stdout.String())
	}

	if strings.Contains(stdout.String(), "\x1b") {
		t.Error("--color auto must not colour output that isn't a terminal")
	}

	stdout.Reset()

	run([]string{"--output", "ascii", "--color", "always", basic}, &stdout, &stderr)

	if !strings.Contains(stdout.String(), "\x1b[48;2;") {
		t.Error("--color always must colour the preview")
	}

	target := filepath.Join(t.TempDir(), "preview.txt")
	stdout.Reset()

	if code := run([]string{"--ascii-output-file", target, basic}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit code = %d", code)
	}

	b, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(b), "MON ") || strings.Contains(string(b), "\x1b") {
		t.Errorf("ASCII file = %q, err = %v (files get colours only with --color always)", b, err)
	}

	if !strings.Contains(stdout.String(), `"schema-version"`) {
		t.Error("with --ascii-output-file, stdout still carries the JSON")
	}
}

func TestJSONIsNotHTMLEscaped(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	run([]string{"../../testdata/golden/warn/repeated-weekday-overlap/input.md"}, &stdout, &stderr)

	if !strings.Contains(stdout.String(), `"text": "> - `) || strings.Contains(stdout.String(), `\u003e`) {
		t.Errorf("excerpts must keep their \">\":\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()

	run([]string{"--error-output-format", "json", "../../testdata/golden/error/time-24-00/input.md"}, &stdout, &stderr)

	if strings.Contains(stderr.String(), `\u003e`) {
		t.Errorf("the JSON report must not HTML-escape:\n%s", stderr.String())
	}
}

// TestCatalogOutputs runs the call the schedule:render task makes: the text form on stdout, and the JSON, the SVG with
// its embedded source and the text form as files.
func TestCatalogOutputs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jsonFile, svgFile, textFile := filepath.Join(
		dir,
		"s.json",
	), filepath.Join(
		dir,
		"s.svg",
	), filepath.Join(
		dir,
		"s.txt",
	)

	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--output", "text", "--json-output-file", jsonFile, "--svg-output-file", svgFile, "--svg-embed-source",
		"--text-output-file", textFile, basic,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}

	var stdoutJSON bytes.Buffer
	run([]string{basic}, &stdoutJSON, &stderr)

	if b, _ := os.ReadFile(jsonFile); !bytes.Equal(b, stdoutJSON.Bytes()) {
		t.Errorf("--json-output-file differs from the JSON on stdout:\n%s", b)
	}

	text, _ := os.ReadFile(textFile)
	if !bytes.Equal(text, stdout.Bytes()) || !strings.Contains(string(text), "~~~text\n") ||
		!strings.Contains(string(text), "> [!SCHEDULE-STATE]") {
		t.Errorf("text form: file %q, stdout %q", text, stdout.String())
	}

	// The text form and the source embedded in the SVG are requests for the same schedule: the same base and week.
	want := scheduleOf(t, stdoutJSON.Bytes())

	if got := scheduleOf(t, evaluateFile(t, dir, "text.md", text)); got != want {
		t.Errorf("text form schedule %s, want %s", got, want)
	}

	if got := scheduleOf(t, evaluateFile(t, dir, "embedded.md", []byte(embedded(t, svgFile)))); got != want {
		t.Errorf("embedded source schedule %s, want %s", got, want)
	}

	if code := run([]string{"--svg-embed-source", basic}, &stdout, &stderr); code != exitInvalid ||
		!strings.Contains(stderr.String(), "--svg-embed-source needs --svg-output-file") {
		t.Errorf("--svg-embed-source alone: exit %d, stderr %q", code, stderr.String())
	}
}

func TestSVGWithoutEmbedHasNoMetadata(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "s.svg")

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--svg-output-file", target, basic}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit code = %d", code)
	}

	if b, _ := os.ReadFile(target); bytes.Contains(b, []byte("<metadata>")) {
		t.Error("the source is embedded only with --svg-embed-source")
	}
}

func evaluateFile(t *testing.T, dir, name string, content []byte) []byte {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{path}, &stdout, &stderr); code != exitOK {
		t.Fatalf("%s: exit %d, stderr %q\n%s", name, code, stderr.String(), content)
	}

	return stdout.Bytes()
}

// scheduleOf returns what a consumer acts on, the base and the week, as compact JSON.
func scheduleOf(t *testing.T, out []byte) string {
	t.Helper()

	var o struct {
		Base string          `json:"base"`
		Week json.RawMessage `json:"week"`
	}

	if err := json.Unmarshal(out, &o); err != nil || o.Base == "" {
		t.Fatalf("no schedule in %s (%v)", out, err)
	}

	var week bytes.Buffer
	if err := json.Compact(&week, o.Week); err != nil {
		t.Fatal(err)
	}

	return o.Base + " " + week.String()
}

// embedded reads the source element from an SVG's metadata with a plain XML parser.
func embedded(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var svg struct {
		Metadata struct {
			Source struct {
				Text string `xml:",chardata"`
			} `xml:"https://github.com/iilei/canned-schedules#svg-source source"`
		} `xml:"metadata"`
	}

	if err := xml.Unmarshal(b, &svg); err != nil {
		t.Fatal(err)
	}

	if svg.Metadata.Source.Text == "" {
		t.Fatalf("no embedded source in %s", b)
	}

	return svg.Metadata.Source.Text
}
