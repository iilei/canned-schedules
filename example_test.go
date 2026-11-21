package cannedschedules_test

import (
	"fmt"

	cs "github.com/iilei/canned-schedules"
)

func Example() {
	src := []byte("# Office Hours\n\n" +
		"> [!SCHEDULE-BASE] Off\n\n" +
		"> [!SCHEDULE-STATE] Medium\n" +
		"> - `08:00` to `18:00` MON-FRI\n")

	r, err := cs.Evaluate(src, &cs.Options{Preview: &cs.PreviewOptions{}})
	if err != nil {
		panic(err) // Only unusable options end here; document problems are diagnostics.
	}

	for _, d := range r.Diagnostics {
		fmt.Println(d)
	}

	if !r.Valid() {
		return
	}

	for _, t := range r.Output.Transitions {
		fmt.Println(t.Time, t.State, t.Cron.POSIX)
	}

	svg, err := r.SVG(cs.SVGOptions{Theme: cs.ThemeLight})
	if err != nil {
		panic(err)
	}

	fmt.Println(len(svg) > 0)
	// Output:
	// 08:00 Medium 0 8 * * 1-5
	// 18:00 Off 0 18 * * 1-5
	// true
}

func ExampleEvaluate_diagnostics() {
	r, err := cs.Evaluate([]byte("# Office Hours\n\n> [!SCHEDULE-STATE] Medium\n> - `08:00` to `24:00` MON\n"),
		nil)
	if err != nil {
		panic(err)
	}

	for _, d := range r.Diagnostics {
		fmt.Println(d.Severity, d.Line, d.Column, d.Code)
	}

	report, err := r.Report("schedule.md").Text()
	if err != nil {
		panic(err)
	}

	fmt.Print(string(report)[:len("schedule.md:4:")])
	// Output:
	// error 4 17 invalid-time
	// schedule.md:4:
}

// A bot that checks every new issue stays silent on text that isn't meant as a schedule. Valid can't tell: a heading
// and prose make a valid, empty schedule.
func ExampleResult_HasDirectives() {
	for _, body := range []string{
		"# A question\n\nWhat does `[!SCHEDULE-STATE]` do?\n",
		"# Office Hours\n\n> [!SCHEDULE-STATE] Medium\n> - `08:00` to `18:00` MON-FRI\n",
	} {
		r, err := cs.Evaluate([]byte(body), nil)
		if err != nil {
			panic(err)
		}

		fmt.Println(r.Valid(), r.HasDirectives())
	}
	// Output:
	// true false
	// true true
}

// A tool that writes a self-hosted playground's site.json checks it first: the page would ignore a broken one.
func ExampleCheckSite() {
	good := `{"args": ["--require-base", "true"], "propose": {"issue": "https://gitlab.example.com/acme/catalog/-/issues/new?issue[description]={content}"}}`
	bad := `{"propose": {"issue": "http://gitlab.example.com/acme/catalog/-/issues/new"}}`

	fmt.Println(len(cs.CheckSite([]byte(good))))

	for _, p := range cs.CheckSite([]byte(bad)) {
		fmt.Println(p)
	}
	// Output:
	// 0
	// propose.issue: needs {content}
	// propose.issue: must be an https URL with a host
}
