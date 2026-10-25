package sphinxkit

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/iilei/canned-schedules/internal/config"
)

const request = "# Office Hours\n\n> [!SCHEDULE:BASE] Minimum\n\n> [!SCHEDULE:STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n"

func TestCSSTargets(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Load(nil)

	for name, tc := range map[string]struct {
		target Target
		want   []string
	}{
		"page": {Page, []string{
			`.rendered.sphinx .admonition.state-2 { --state: `,
			`:root[data-theme="dark"] .rendered.sphinx .admonition.state-2`,
			`:root[data-theme="system"] .rendered.sphinx .admonition.state-2`,
		}},
		"furo": {Furo, []string{
			"\n.admonition.state-2 { --state: ",
			`body[data-theme="dark"] .admonition.state-2`,
			`@media (prefers-color-scheme: dark) { body:not([data-theme="light"]) .admonition.state-2`,
		}},
	} {
		css, err := CSS(cfg, tc.target)
		if err != nil {
			t.Fatal(err)
		}

		for _, w := range append(tc.want,
			"--color-admonition-title: var(--state", "--icon-admonition-default", `url("fonts/ubuntu-mono-400.woff2")`,
			`url("fonts/ubuntu-700.woff2")`, `font-style: italic; font-display: swap; src: url("fonts/ubuntu-mono-400-italic.woff2")`,
			`.admonition.schedule code { font-family: "Canned Schedules Mono"`) {
			if !strings.Contains(css, w) {
				t.Errorf("%s: missing %q", name, w)
			}
		}

		for i := range cfg.States {
			c := cfg.Style(i).Color
			if !strings.Contains(css, c.Light) || !strings.Contains(css, c.Dark) {
				t.Errorf("%s: state %d colours missing", name, i)
			}
		}

		if strings.Contains(css, "body {") || strings.Contains(css, "html {") {
			t.Errorf("%s: fonts and styles must stay inside schedule admonitions", name)
		}
	}
}

func TestKit(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Load(nil)

	a, err := Kit([]byte(request), cfg, "canned-schedules test")
	if err != nil {
		t.Fatal(err)
	}

	b, _ := Kit([]byte(request), cfg, "canned-schedules test")
	if !bytes.Equal(a, b) {
		t.Error("the same input must give the same zip")
	}

	files := unzip(t, a)

	var names []string
	for name := range files {
		names = append(names, strings.TrimPrefix(name, root))
	}

	slices.Sort(names)

	want := []string{
		"README.md",
		"VERSION",
		"_ext/canned_schedules/__init__.py",
		"_ext/canned_schedules/static/canned-schedules.css",
		"_ext/canned_schedules/static/fonts/UFL.txt",
		"_ext/canned_schedules/static/fonts/ubuntu-400.woff2",
		"_ext/canned_schedules/static/fonts/ubuntu-500.woff2",
		"_ext/canned_schedules/static/fonts/ubuntu-700.woff2",
		"_ext/canned_schedules/static/fonts/ubuntu-mono-400-italic.woff2",
		"_ext/canned_schedules/static/fonts/ubuntu-mono-400.woff2",
		"_ext/canned_schedules/static/fonts/ubuntu-mono-700.woff2",
		"examples/office-hours.md",
	}
	if !slices.Equal(names, want) {
		t.Errorf("zip contents:\n got %q\nwant %q", names, want)
	}

	if !strings.Contains(files[root+"README.md"], `extensions += ["canned_schedules"]`) ||
		!strings.Contains(files[root+"examples/office-hours.md"], ":::{admonition} Medium") ||
		!strings.HasPrefix(files[root+"VERSION"], "canned-schedules test\nconfig sha256:") {
		t.Error("README, example or VERSION content is off")
	}

	if _, err := Kit([]byte("no heading\n"), cfg, "x"); err == nil {
		t.Error("an invalid request must not produce a kit")
	}
}

// TestExtensionIsPython compiles the generated Sphinx extension when python3 is available.
func TestExtensionIsPython(t *testing.T) {
	t.Parallel()

	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}

	cfg, _ := config.Load(nil)

	kit, err := Kit([]byte(request), cfg, "canned-schedules test")
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "__init__.py")
	if err := os.WriteFile(path, []byte(unzip(t, kit)[root+"_ext/canned_schedules/__init__.py"]), 0o600); err != nil {
		t.Fatal(err)
	}

	if out, err := exec.CommandContext(t.Context(), python, "-m", "py_compile", path).CombinedOutput(); err != nil {
		t.Errorf("the extension doesn't compile: %v\n%s", err, out)
	}
}

func unzip(t *testing.T, data []byte) map[string]string {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	files := map[string]string{}

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}

		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		files[f.Name] = string(b)
	}

	return files
}
