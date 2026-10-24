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
// result holds one tier id per minute, starting MON 00:00.
func paint(base config.Tier, ranges []tierRange) []string {
	week := make([]string, len(expr.Weekdays)*minutesPerDay)

	for i := range week {
		week[i] = base.ID
	}

	for _, r := range ranges {
		length := (r.End - r.Start + minutesPerDay) % minutesPerDay
		if length == 0 {
			length = minutesPerDay // Start == end means 24 hours.
		}

		for _, d := range r.Days {
			start := d*minutesPerDay + r.Start
			for i := range length {
				week[(start+i)%len(week)] = r.tier.ID
			}
		}
	}

	return week
}

// weekRuns splits the painted week into per-day runs.
func weekRuns(week []string) Week {
	runs := make([][]Run, len(expr.Weekdays))
	for d := range runs {
		runs[d] = dayRuns(week[d*minutesPerDay : (d+1)*minutesPerDay])
	}

	return Week{MON: runs[0], TUE: runs[1], WED: runs[2], THU: runs[3], FRI: runs[4], SAT: runs[5], SUN: runs[6]}
}

// dayRuns merges consecutive minutes with the same tier into half-open runs. The last run ends at "00:00".
func dayRuns(day []string) []Run {
	var runs []Run

	start := 0
	for m := 1; m <= len(day); m++ {
		if m < len(day) && day[m] == day[start] {
			continue
		}

		runs = append(runs, Run{Start: clock(start), End: clock(m % minutesPerDay), Tier: day[start]})
		start = m
	}

	return runs
}

func clock(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/minutesPerHour, minutes%minutesPerHour)
}
