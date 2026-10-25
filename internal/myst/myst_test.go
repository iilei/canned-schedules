package myst

import (
	"errors"
	"strings"
	"testing"

	"github.com/iilei/canned-schedules/internal/config"
)

func TestRender(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Load(nil)
	src := "# Office Hours\n\nSome prose that stays in the issue.\n\n" +
		"> [!SCHEDULE-META] Weekdays\n> - x-owner: `ops`\n\n" +
		"> [!SCHEDULE-BASE] Minimum\n\n" +
		"> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n> - `10:00` to `18:00` SAT\n\n" +
		"> [!NOTE]\n> Also left out.\n"

	got, err := Render([]byte(src), cfg)
	if err != nil {
		t.Fatal(err)
	}

	want := "# Office Hours\n" +
		"\n:::{admonition} Schedule: Weekdays\n:class: schedule schedule-meta\n- x-owner: `ops`\n:::\n" +
		"\n:::{admonition} Base state: Minimum\n:class: schedule schedule-base state-1\n" + BaseText + "\n:::\n" +
		"\n:::{admonition} Medium\n:class: schedule schedule-state state-2\n" +
		"- `06:00` to `22:00` MON-FRI\n" +
		"- `10:00` to `18:00` SAT\n:::\n"

	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderRejectsInvalid(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Load(nil)
	if _, err := Render([]byte("no heading\n"), cfg); !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}

func TestDisplayNamesStayOnOneLine(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Load([]config.Override{{Path: "states[2].display-name", Value: `"Medium\n:class: evil"`}})

	got, err := Render([]byte("# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON\n"), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if want := ":::{admonition} Medium\\u000a:class: evil\n:class: schedule schedule-state state-2\n"; !strings.Contains(
		got,
		want,
	) {
		t.Errorf("a display name must not inject directive options:\n%s", got)
	}
}

func TestHintWithoutItemsIsAnAdmonition(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Load(nil)

	got, err := Render([]byte("# Shop\n\n> [!SCHEDULE-META] Weekend shop\n\n> [!SCHEDULE-BASE] Off\n"), cfg)
	if err != nil {
		t.Fatal(err)
	}

	want := "# Shop\n\n:::{admonition} Schedule: Weekend shop\n:class: schedule schedule-meta schedule-empty\n" + EmptyBody + "\n:::\n" +
		"\n:::{admonition} Base state: Off\n:class: schedule schedule-base state-0\n" + BaseText + "\n:::\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
