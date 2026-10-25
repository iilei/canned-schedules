package canonical

import (
	"errors"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/iilei/canned-schedules/internal/config"
)

func TestRender(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Load(nil)
	src := "#  Office Hours \n\nProse that is dropped, with a [link](https://example.com).\n\n" +
		"> [!schedule-meta]  Weekdays only \n> - x-owner :  `ops`\n> - schema-version: `1`\n\n" +
		"> [!NOTE]\n> Dropped as well.\n\n" +
		"> [!Schedule-Base] minimum\n\n" +
		"> [!SCHEDULE-STATE] MEDIUM\n> - `6:00` to `22:00` MON - FRI\n> - `10:00` to `0:00`  SAT, SUN\n\n" +
		"> [!SCHEDULE-STATE] Maximum\n> - `22:00` to `2:00` MON-WED,TUE\n"

	got, err := Render([]byte(src), cfg)
	if err != nil {
		t.Fatal(err)
	}

	want := "# Office Hours\n" +
		"\n> [!SCHEDULE-META] Weekdays only\n> - x-owner: `ops`\n> - schema-version: `1`\n" +
		"\n> [!SCHEDULE-BASE] Minimum\n" +
		"\n> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n> - `10:00` to `00:00` SAT,SUN\n" +
		"\n> [!SCHEDULE-STATE] Maximum\n> - `22:00` to `02:00` MON-WED,TUE\n"

	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderMinimal(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Load(nil)

	got, err := Render([]byte("# Always Off\n\n> [!SCHEDULE-META] Nothing scheduled\n"), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if want := "# Always Off\n\n> [!SCHEDULE-META] Nothing scheduled\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderRejectsInvalid(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Load(nil)
	if _, err := Render([]byte("no heading\n"), cfg); !errors.Is(err, ErrInvalid) {
		t.Errorf("got %v, want ErrInvalid", err)
	}
}

// TestCodeSpan checks that every value comes back unchanged from the code span goldmark parses.
func TestCodeSpan(t *testing.T) {
	t.Parallel()

	for _, v := range []string{"ops", "a`b", "``x", "x`", "`", " lead", "trail ", " both ", "  ", "a  b", "OPS-1234"} {
		src := []byte(codeSpan(v))
		doc := goldmark.New().Parser().Parse(text.NewReader(src))

		var got strings.Builder

		_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if t, ok := n.(*ast.Text); ok && entering && n.Parent().Kind() == ast.KindCodeSpan {
				got.Write(t.Segment.Value(src))
			}

			return ast.WalkContinue, nil
		})

		if got.String() != v {
			t.Errorf("codeSpan(%q) = %q parses as %q", v, src, got.String())
		}
	}
}

func TestText(t *testing.T) {
	t.Parallel()

	c := "# Name\n\n> [!SCHEDULE-BASE] Off\n"

	want := "# Name\n\n~~~text\nName\nMON ...\n~~~\n\n> [!SCHEDULE-BASE] Off\n"
	if got := Text(c, []byte("Name\nMON ...")); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// A preview line starting with tildes gets a longer fence.
	if got := Text(c, []byte("~~~~ odd\n")); !strings.Contains(got, "\n~~~~~text\n~~~~ odd\n~~~~~\n") {
		t.Errorf("fence not lengthened: %q", got)
	}
}
