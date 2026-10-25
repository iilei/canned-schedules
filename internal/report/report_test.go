package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/schema"
)

const src = "# Plan\n\n> [!SCHEDULE-STATE] Medium\n> - `10:00` to `24:00` SAT\n\t\tx [!SCHEDULE-BASE]\n"

func sample() Report {
	return Build("requests/office.md", []byte(src), diag.List{
		diag.Errorf(diag.Pos{}, diag.CodeInvalidFlag, "--sample-interval 7: must divide 60"),
		diag.Errorf(diag.Pos{Line: 4, Col: 17}, diag.CodeInvalidTime, "24:00 is not a valid time"),
		diag.Errorf(diag.Pos{Line: 5, Col: 6}, diag.CodeMisplacedDirective, "misplaced"),
		diag.Warnf(diag.Pos{}, diag.CodeStyleReuse, "styles repeat"),
	})
}

func TestBuild(t *testing.T) {
	t.Parallel()

	r := sample()
	if r.Summary != (Summary{Errors: 3, Warnings: 1}) {
		t.Errorf("summary = %+v", r.Summary)
	}

	noPos := r.Diagnostics[0]
	if noPos.Location != "" || noPos.Line != 0 || noPos.Excerpt != nil || noPos.File != "requests/office.md" {
		t.Errorf("diagnostic without position = %+v", noPos)
	}

	e := r.Diagnostics[1]
	if e.Location != "requests/office.md:4:17" || e.Excerpt == nil {
		t.Fatalf("entry = %+v", e)
	}

	if e.Excerpt.Text != "> - `10:00` to `24:00` SAT" || e.Excerpt.Marker != strings.Repeat(" ", 16)+"^" {
		t.Errorf("excerpt = %+v", e.Excerpt)
	}

	if got := r.Diagnostics[2].Excerpt.Marker; got != "\t\t   ^" {
		t.Errorf("tabs before the column must be kept in the marker, got %q", got)
	}
}

func TestBuildSanitizes(t *testing.T) {
	t.Parallel()

	evil := "evil\n::error::x\u202e.md"
	r := Build(evil, []byte("a\x1b[31mb [!X:Y]\n"), diag.List{
		diag.Errorf(diag.Pos{Line: 1, Col: 9}, diag.CodeMisplacedDirective, "bad \u2028 message"),
	})

	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	for _, bad := range []string{"\n::", "\u202e", "\x1b", "\u2028"} {
		if strings.Contains(string(raw), bad) {
			t.Errorf("report contains unsanitized %q: %s", bad, raw)
		}
	}

	e := r.Diagnostics[0]
	if !strings.HasPrefix(e.Location, `evil\u000a::error::x\u202e.md:1:9`) {
		t.Errorf("location = %q", e.Location)
	}

	if want := len([]rune(`a\u001b[31mb `)); len(e.Excerpt.Marker) != want+1 {
		t.Errorf("marker %q must line up with the sanitized excerpt %q", e.Excerpt.Marker, e.Excerpt.Text)
	}
}

func TestExcerptSkipsFenceLines(t *testing.T) {
	t.Parallel()

	r := Build("f.md", []byte("~~~~~~ [!X:Y]\n"), diag.List{diag.Errorf(diag.Pos{Line: 1, Col: 10}, "c", "m")})
	if r.Diagnostics[0].Excerpt != nil {
		t.Error("a line that could close the Markdown fence must not become an excerpt")
	}
}

func TestReportsMatchSchema(t *testing.T) {
	t.Parallel()

	sch, err := schema.Compile(schema.ReportSchemaURL, schema.ReportSchema)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range []Report{sample(), Build("ok.md", []byte("# Ok\n"), nil), Build("", nil, diag.List{
		diag.Errorf(diag.Pos{}, diag.CodeInvalidArgs, "missing input file"),
	})} {
		var buf bytes.Buffer
		if err := Render(&buf, r, FormatJSON, ""); err != nil {
			t.Fatal(err)
		}

		doc, err := jsonschema.UnmarshalJSON(&buf)
		if err != nil {
			t.Fatal(err)
		}

		if err := sch.Validate(doc); err != nil {
			t.Errorf("report does not match its schema: %v", err)
		}
	}
}

func TestRender(t *testing.T) {
	t.Parallel()

	r := sample()

	var txt bytes.Buffer
	if err := Render(&txt, r, "", ""); err != nil {
		t.Fatal(err)
	}

	wantTxt := "requests/office.md: error[invalid-flag]: --sample-interval 7: must divide 60\n" +
		"requests/office.md:4:17: error[invalid-time]: 24:00 is not a valid time\n"
	if !strings.HasPrefix(txt.String(), wantTxt) {
		t.Errorf("txt =\n%s", txt.String())
	}

	var md bytes.Buffer
	if err := Render(&md, r, "", TemplateMarkdownGFM); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"**Errors:** 3 · **Warnings:** 1",
		"#### error `invalid-time`\n\n~~~~~~text\nrequests/office.md:4:17: 24:00 is not a valid time\n\n" +
			"> - `10:00` to `24:00` SAT\n                ^\n~~~~~~",
		"#### warn `style-reuse`\n\n~~~~~~text\nrequests/office.md: styles repeat\n~~~~~~",
	} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("markdown lacks %q:\n%s", want, md.String())
		}
	}

	var empty bytes.Buffer
	if err := Render(&empty, Build("ok.md", nil, nil), "", TemplateMarkdownGFM); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(empty.String(), "No problems found.") {
		t.Errorf("empty report = %q", empty.String())
	}
}

func TestExamples(t *testing.T) {
	t.Parallel()

	r := Build("f.md", []byte("x\n"), diag.List{
		diag.Errorf(diag.Pos{Line: 1, Col: 1}, diag.CodeInvalidTime, "24:00 is not a valid time").
			WithExample("`21:45` to `00:00`"),
	})

	if r.Diagnostics[0].Example != "`21:45` to `00:00`" {
		t.Fatalf("entry = %+v", r.Diagnostics[0])
	}

	for format, want := range map[string]string{
		FormatTXT:  "f.md:1:1: error[invalid-time]: 24:00 is not a valid time; e.g. `21:45` to `00:00`\n",
		FormatJSON: `"example": "` + "`21:45` to `00:00`" + `"`,
	} {
		var buf bytes.Buffer
		if err := Render(&buf, r, format, ""); err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(buf.String(), want) {
			t.Errorf("%s output lacks %q:\n%s", format, want, buf.String())
		}
	}

	var md bytes.Buffer
	if err := Render(&md, r, "", TemplateMarkdownGFM); err != nil {
		t.Fatal(err)
	}

	want := "~~~~~~\n\nExample:\n\n~~~~~~markdown\n`21:45` to `00:00`\n~~~~~~"
	if !strings.Contains(md.String(), want) {
		t.Errorf("markdown lacks the example fence:\n%s", md.String())
	}

	if strings.Contains(md.String(), "e.g.") {
		t.Errorf("markdown must not inline the example:\n%s", md.String())
	}
}
