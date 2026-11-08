package schedule

import (
	"fmt"

	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/expr"
)

const (
	minutesPerHour = 60
	minutesPerDay  = 24 * minutesPerHour
)

// paint fills every minute of the week with base, then paints the ranges in source order: later ranges win where
// they overlap. A range belongs to its start day and spills past midnight (and from SUN into MON) as needed. The
// result holds one state id per minute, starting MON 00:00.
func paint(base config.State, ranges []stateRange) []string {
	owners := paintOwners(ranges)
	week := make([]string, len(owners))

	for i, o := range owners {
		week[i] = base.ID
		if o >= 0 {
			week[i] = ranges[o].state.ID
		}
	}

	return week
}

// paintOwners paints range indexes instead of states: every minute of the week holds the index of the range that
// paints it last, or -1 where the base shows. paint and the trace share it, so they can't disagree.
func paintOwners(ranges []stateRange) []int {
	owners := make([]int, minutesPerWeek)

	for i := range owners {
		owners[i] = -1
	}

	for i, r := range ranges {
		length := r.length()

		for _, d := range r.Days {
			start := d*minutesPerDay + r.Start
			for m := range length {
				owners[(start+m)%minutesPerWeek] = i
			}
		}
	}

	return owners
}

// weekRuns splits the painted week into per-day runs.
func weekRuns(week []string) Week {
	runs := make([][]Run, len(expr.Weekdays))
	for d := range runs {
		runs[d] = dayRuns(week[d*minutesPerDay : (d+1)*minutesPerDay])
	}

	return Week{MON: runs[0], TUE: runs[1], WED: runs[2], THU: runs[3], FRI: runs[4], SAT: runs[5], SUN: runs[6]}
}

// dayRuns merges consecutive minutes with the same state into half-open runs. The last run ends at "00:00".
func dayRuns(day []string) []Run {
	var runs []Run

	start := 0
	for m := 1; m <= len(day); m++ {
		if m < len(day) && day[m] == day[start] {
			continue
		}

		runs = append(runs, Run{Start: clock(start), End: clock(m % minutesPerDay), State: day[start]})
		start = m
	}

	return runs
}

func clock(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/minutesPerHour, minutes%minutesPerHour)
}
