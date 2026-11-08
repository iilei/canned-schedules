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
