package schedule

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/iilei/canned-schedules/internal/expr"
)

// sundayFirst is the day order both cron dialects number: POSIX 0 = SUN, AWS names SUN…SAT (1 = SUN).
var sundayFirst = []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}

// Transition is a moment when the active state changes. All days that switch to the same state at the same time are
// grouped. Time, Days and State are the facts; Cron renders them for schedulers.
type Transition struct { //nolint:govet // fieldalignment: field order is the JSON key order people read.
	Time  string   `json:"time"`
	Days  []string `json:"days"`
	State string   `json:"state"`
	Cron  Cron     `json:"cron"`
}

// Occurrence is one switch of the active state on one weekday, so every day has its own entry and its own cron.
type Occurrence struct {
	Dow   string `json:"dow"`
	Time  string `json:"time"`
	State string `json:"state"`
	Cron  Cron   `json:"cron"`
}

// Cron holds the transition in two cron dialects. Neither carries a time zone: they are wall-clock times.
type Cron struct {
	// POSIX is five-field Unix cron with numeric weekdays (0 = SUN), as used by EC2 Auto Scaling scheduled actions.
	POSIX string `json:"posix"`
	// AWS is the six-field EventBridge format with weekday names and a year field.
	AWS string `json:"aws"`
}

// transitionGroup collects the minutes of the week at which the week switches to state at a given time of day.
type transitionGroup struct {
	state   string
	minutes []int
	time    int
}

// transitions finds every minute where the state changes (the week is cyclic: MON 00:00 follows SUN 23:59), groups
// them by time of day and state, and orders the groups by their first minute in the week, then by state.
func transitions(week []string) []Transition {
	type key struct {
		state string
		time  int
	}

	index := map[key]int{}

	var groups []transitionGroup

	for m := range week {
		prev := week[(m-1+len(week))%len(week)]
		if week[m] == prev {
			continue
		}

		k := key{state: week[m], time: m % minutesPerDay}
		if i, ok := index[k]; ok {
			groups[i].minutes = append(groups[i].minutes, m)

			continue
		}

		index[k] = len(groups)
		groups = append(groups, transitionGroup{state: k.state, time: k.time, minutes: []int{m}})
	}

	slices.SortStableFunc(groups, func(a, b transitionGroup) int {
		return cmp.Or(cmp.Compare(a.minutes[0], b.minutes[0]), cmp.Compare(a.state, b.state))
	})

	out := make([]Transition, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.transition())
	}

	return out
}

// occurrences lists every minute where the state changes, in the order of the week (MON 00:00 first). The week is
// cyclic, so the first entry follows the last one.
func occurrences(week []string) []Occurrence {
	out := []Occurrence{}

	for m := range week {
		if week[m] == week[(m-1+len(week))%len(week)] {
			continue
		}

		day := expr.Weekdays[m/minutesPerDay]
		out = append(out, Occurrence{
			Dow: day, Time: clock(m % minutesPerDay), State: week[m],
			Cron: cronAt(m%minutesPerDay, []int{slices.Index(sundayFirst, day)}),
		})
	}

	return out
}

func (g transitionGroup) transition() Transition {
	var days []int
	for _, m := range g.minutes {
		days = append(days, m/minutesPerDay)
	}

	slices.Sort(days)
	days = slices.Compact(days)

	names := make([]string, 0, len(days))
	sunFirst := make([]int, 0, len(days))

	for _, d := range days {
		names = append(names, expr.Weekdays[d])
		sunFirst = append(sunFirst, slices.Index(sundayFirst, expr.Weekdays[d]))
	}

	slices.Sort(sunFirst)

	return Transition{Time: clock(g.time), Days: names, State: g.state, Cron: cronAt(g.time, sunFirst)}
}

// cronAt renders a time of day on sorted Sunday-first day indexes in both dialects.
func cronAt(time int, sunFirst []int) Cron {
	h, m := strconv.Itoa(time/minutesPerHour), strconv.Itoa(time%minutesPerHour)

	return Cron{
		POSIX: m + " " + h + " * * " + compress(sunFirst, strconv.Itoa),
		AWS:   m + " " + h + " ? * " + compress(sunFirst, func(i int) string { return sundayFirst[i] }) + " *",
	}
}

// compress renders sorted Sunday-first day indexes as ranges and lists ("1-5", "0-1,5-6"). It never wraps around
// the week, because wrapped ranges aren't portable across cron implementations. All seven days become "*".
func compress(days []int, label func(int) string) string {
	if len(days) == len(sundayFirst) {
		return "*"
	}

	var parts []string

	for i := 0; i < len(days); {
		j := i
		for j+1 < len(days) && days[j+1] == days[j]+1 {
			j++
		}

		part := label(days[i])
		if j > i {
			part += "-" + label(days[j])
		}

		parts = append(parts, part)
		i = j + 1
	}

	return strings.Join(parts, ",")
}
