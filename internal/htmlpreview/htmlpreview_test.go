package htmlpreview

import (
	"strings"
	"testing"

	"github.com/iilei/canned-schedules/internal/config"
)

const doc = "# Plan\n\n" +
	"> [!SCHEDULE-META] Office\n> - x-owner: `ops`\n\n" +
	"> [!SCHEDULE-BASE] Minimum\n\n" +
	"> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n\n" +
	"> [!NOTE]\n> Mention MON here.\n\n" +
	"> [!TIP] with a title\n> text\n\n" +
	"> [!OTHER:THING] x\n\n" +
	"Prose with MON, <script>alert(1)</script> and [a link](javascript:alert(1)).\n"

func render(t *testing.T, mode string) string {
	t.Helper()

	cfg, _ := config.Load(nil)

	html, err := Render([]byte(doc), cfg, mode)
	if err != nil {
		t.Fatal(err)
	}

	return html
}

func TestGitHub(t *testing.T) {
	t.Parallel()

	html := render(t, GitHub)

	for _, want := range []string{
		`<div class="markdown-alert markdown-alert-note"><p class="markdown-alert-title">Note</p>`,
		`<blockquote>` + "\n" + `<p>[!SCHEDULE-STATE] Medium</p>`, // GitHub shows schedule directives literally.
		`<p>[!TIP] with a title`, // A marker with a title is no GitHub alert.
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in:\n%s", want, html)
		}
	}
}

func TestSphinx(t *testing.T) {
	t.Parallel()

	html := render(t, Sphinx)

	for _, want := range []string{
		`<div class="admonition schedule schedule-meta"><p class="admonition-title">Schedule: Office</p>`,
		`<div class="admonition schedule schedule-base state-1"><p class="admonition-title">Base state: Minimum</p>`,
		`<div class="admonition schedule schedule-state state-2"><p class="admonition-title">Medium</p>`,
		`<code>06:00</code> to <code>22:00</code> MON-FRI`,
		`<div class="admonition note"><p class="admonition-title">Note</p>`,
		`<div class="admonition tip"><p class="admonition-title">Tip</p>`,
		`<div class="admonition"><p class="admonition-title">OTHER:THING x</p>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in:\n%s", want, html)
		}
	}

	if strings.Contains(html, "<abbr") {
		t.Error("weekdays are written as in the request, without tooltips")
	}

	if strings.Contains(html, "[!SCHEDULE-") {
		t.Error("markers must not show in the Sphinx view")
	}
}

func TestMetaWithoutItemsIsAnAdmonition(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Load(nil)

	html, err := Render([]byte("# Shop\n\n> [!SCHEDULE-META] Weekend shop\n"), cfg, Sphinx)
	if err != nil {
		t.Fatal(err)
	}

	want := `<div class="admonition schedule schedule-meta schedule-empty"><p class="admonition-title">Schedule: Weekend shop</p>` +
		"\n<p>&nbsp;</p>\n</div>"
	if !strings.Contains(html, want) {
		t.Errorf("a META block with only a summary must render as an admonition, like the MyST output:\n%s", html)
	}
}

func TestSafety(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{GitHub, Sphinx} {
		html := render(t, mode)
		if strings.Contains(html, "<script") || strings.Contains(html, "javascript:") {
			t.Errorf("%s: raw HTML or a dangerous link got through:\n%s", mode, html)
		}
	}
}
