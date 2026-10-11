package schedules

import "strings"

import "list"

_slots:        48
_slot_minutes: div(24*60, _slots)

#schedule_day: strings.MinRunes(_slots) & strings.MaxRunes(_slots)
#weekly_schedule: {
	MON: #schedule_day
	TUE: #schedule_day
	WED: #schedule_day
	THU: #schedule_day
	FRI: #schedule_day
	SAT: #schedule_day
	SUN: #schedule_day
}

#schedule_contract: {
	capacity_tiers: {
		[string]: {
			min:     int
			desired: int
			max:     int
			color:   =~"^#[0-9A-Fa-f]{6}$"
		}
	}
	capacity_legend: list.MinItems(1) & [...{
		symbol: strings.MinRunes(1) & strings.MaxRunes(1)
		tier:   string
	}]
	schedule_theme: {
		background: =~"^#[0-9A-Fa-f]{6}$"
		label:      =~"^#[0-9A-Fa-f]{6}$"
	}
	capacity_schedules: {
		[string]: #weekly_schedule
	}
	...

	_capacity_tier_names: [for tier, _ in capacity_tiers {tier}]
	_legend_symbols: [for entry in capacity_legend {entry.symbol}]
	_legend_symbols_are_unique: [
		for index, entry in capacity_legend
		for other_index, other_entry in capacity_legend
		if other_index > index {
			true & (entry.symbol != other_entry.symbol)
		}
	]
	_legend_tiers_exist: [
		for entry in capacity_legend {
			true & list.Contains(_capacity_tier_names, entry.tier)
		}
	]
	_schedule_symbols_are_known: [
		for _, schedule in capacity_schedules
		for _, day in schedule
		for _, symbol in strings.Split(day, "") {
			true & list.Contains(_legend_symbols, symbol)
		}
	]
	_week_boundary_is_continuous: [
		for _, schedule in capacity_schedules {
			let monday = strings.Split(schedule.MON, "")
			let sunday = strings.Split(schedule.SUN, "")
			true & (monday[0] == sunday[_slots-1])
		}
	]
}
