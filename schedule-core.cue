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
_day_minimum: _t1 * _slots
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
		color:   "#e0e0e0"
	}

	Minimum: {
		min:     1 & <=desired
		desired: 2 & <=(max * 2)
		max:     desired*2 & >=(desired * 2)
		color:   "#90ee90"
	}

	Medium: {
		min:     1 & <=desired
		desired: 2 & <=(max * 2)
		max:     desired*2 & >=(desired * 2)
		color:   "#228b22"
	}

	Maximum: {
		min:     4 & <=desired
		desired: 4 & <=(max * 2)
		max:     desired*2 & >=(desired * 2)
		color:   "#006400"
	}
}
capacity_legend: [
	{symbol: _t0, tier: "Off"},
	{symbol: _t1, tier: "Minimum"},
	{symbol: _t2, tier: "Medium"},
	{symbol: _t3, tier: "Maximum"},
]
schedule_theme: {
	background: "#1a1a2e"
	label:      "#cccccc"
}

_schedule_validation: #schedule_contract & {
	capacity_tiers:     capacity_tiers
	capacity_legend:    capacity_legend
	capacity_schedules: capacity_schedules
}

_symbol_tiers: {
	for entry in capacity_legend {
		"\(entry.symbol)": entry.tier
	}
}

aws_cron: {
	timezone: "UTC"
	schedules: {
		for schedule_name, schedule in capacity_schedules {
			"\(schedule_name)": [
				for absolute_slot in list.Range(0, _slots*len(_days), 1)
				let day_index = div(absolute_slot, _slots)
				let slot_index = mod(absolute_slot, _slots)
				let previous_absolute_slot = mod(absolute_slot+_slots*len(_days)-1, _slots*len(_days))
				let day = _days[day_index]
				let previous_day = _days[div(previous_absolute_slot, _slots)]
				let previous_slot = mod(previous_absolute_slot, _slots)
				let current_symbol = strings.Split(schedule[day], "")[slot_index]
				let previous_symbol = strings.Split(schedule[previous_day], "")[previous_slot]
				let current_tier = _symbol_tiers[current_symbol]
				let previous_tier = _symbol_tiers[previous_symbol]
				if current_tier != previous_tier
				let matching_days = [
					for candidate_day_index, candidate_day in _days
					let candidate_absolute_slot = candidate_day_index*_slots + slot_index
					let candidate_previous_absolute_slot = mod(candidate_absolute_slot+_slots*len(_days)-1, _slots*len(_days))
					let candidate_previous_day = _days[div(candidate_previous_absolute_slot, _slots)]
					let candidate_previous_slot = mod(candidate_previous_absolute_slot, _slots)
					let candidate_symbol = strings.Split(schedule[candidate_day], "")[slot_index]
					let candidate_previous_symbol = strings.Split(schedule[candidate_previous_day], "")[candidate_previous_slot]
					if _symbol_tiers[candidate_symbol] == current_tier &&
						_symbol_tiers[candidate_previous_symbol] == previous_tier {
						candidate_day
					}
				]
				if day == matching_days[0]
				let joined_days = strings.Join(matching_days, ",") {
					minute:      mod(slot_index*_slot_minutes, 60)
					hour:        div(slot_index*_slot_minutes, 60)
					days:        matching_days
					from_tier:   previous_tier
					to_tier:     current_tier
					day_of_week: string
					if joined_days == "MON,TUE,WED,THU,FRI" {
						day_of_week: "MON-FRI"
					}
					if joined_days == "SAT,SUN" {
						day_of_week: "SAT-SUN"
					}
					if joined_days == "MON,TUE,WED,THU,FRI,SAT,SUN" {
						day_of_week: "MON-SUN"
					}
					if joined_days != "MON,TUE,WED,THU,FRI" &&
						joined_days != "SAT,SUN" &&
						joined_days != "MON,TUE,WED,THU,FRI,SAT,SUN" {
						day_of_week: joined_days
					}
					cron: "\(mod(slot_index*_slot_minutes, 60)) \(div(slot_index*_slot_minutes, 60)) ? * \(day_of_week) *"
				}
			]
		}
	}
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
