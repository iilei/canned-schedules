// Welcome to CUE!
//
// Play around with CUE by typing directly into this window,
// or use the Examples menu to load some CUE.
//
// There are many examples of how to use the language on the CUE
// documentation site, https://cuelang.org/docs/, with the language
// tour being a great place to start: https://cuelang.org/docs/tour/.
//
// You can safely delete the text in this editor window - simply
// reload the page to see this message again. Happy CUEing!

// see https://cuelang.org/docs/tour/types/templates/

package schedules

import "strings"

import "list"

import "math"

// Reusable building blocks
_t0: "\u2591" // ░
_t1: "\u2592" // ▒
_t2: "\u2593" // ▓
_t3: "\u2588" // █

_days: ["MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"]
_day_maximum: _t1*10 + _t2*4 + _t3*20 + _t2*4 + _t1*10
_day_medium:  _t1*10 + _t2*28 + _t1*10
_all_off:     _t0 * _slots

#base: {
	MON: _day_maximum
	TUE: _day_maximum
	WED: _day_maximum
	THU: _day_maximum
	FRI: _day_maximum
	SAT: _day_medium
	SUN: _all_off
	...
}

time_brackets: list.MinItems(_slots) &
	list.MaxItems(_slots) &
	[
		for minute in list.Range(0, 24*60, (24/_slots)*60) {
			int & >=0 & <(24 * 60) & minute
		}
	]
// ------------------------------------------------------------------------------

capacity_tiers: {
	Off: {
		min:     0 & <=desired
		desired: 0 & <=(max * 2)
		max:     desired*2 & >=(desired * 2)
	}

	Minimum: {
		min:     1 & <=desired
		desired: 2 & <=(max * 2)
		max:     desired*2 & >=(desired * 2)
	}

	Medium: {
		min:     1 & <=desired
		desired: 2 & <=(max * 2)
		max:     desired*2 & >=(desired * 2)
	}

	Maximum: {
		min:     4 & <=desired
		desired: 4 & <=(max * 2)
		max:     desired*2 & >=(desired * 2)
	}
}
capacity_legend: [
	{symbol: _t0, tier: "Off"},
	{symbol: _t1, tier: "Minimum"},
	{symbol: _t2, tier: "Medium"},
	{symbol: _t3, tier: "Maximum"},
]

_schedule_validation: #schedule_contract & {
	capacity_tiers:     capacity_tiers
	capacity_legend:    capacity_legend
	capacity_schedules: capacity_schedules
}
// rendered output:
// $ cue export . > test.cue.json
// ------------------------------------------------------------------------------

time_scale_ruler: strings.MinRunes(_slots) & strings.MaxRunes(_slots) & strings.Join([
	for slot in list.Range(0, _slots, 1) {
		if math.Mod(slot*2, _slots) == 0 {
			"║"
		}
		if math.Mod(slot*2, _slots) != 0 && math.Mod(slot*4, _slots) == 0 {
			"|"
		}
		if math.Mod(slot*4, _slots) != 0 && math.Mod(slot*8, _slots) == 0 {
			":"
		}
		if math.Mod(slot*8, _slots) != 0 {
			" "
		}
	}
], "")

time_scale_annotations: [
	for minute in time_brackets {
		let hour = div(minute, 60)
		let remainder = mod(minute, 60)
		strings.Join([
			if hour < 10 {"0"},
			"\(hour)",
			":",
			if remainder < 10 {"0"},
			"\(remainder)",
		], "")
	}
]

// ----------------------------------------------------------------------------------------------------------------------------
capacity_schedules: {
	[string]: #weekly_schedule
	int: {
		for day in _days
		if !list.Contains(["SAT", "SUN"], day) {
			"\(day)": _day_maximum
		}
		"SAT": _day_medium
		"SUN": _all_off
	}
}
