package schedules

// Reusable weekly patterns; only capacity_schedules is exported.
_base_schedules: {
	standard: #base
	weekday_maximum: {
		for day in _days
		if day != "SAT" && day != "SUN" {
			"\(day)": _day_maximum
		}
		SAT: _all_off
		SUN: _all_off
	}
	weekday_medium: {
		for day in _days
		if day != "SAT" && day != "SUN" {
			"\(day)": _day_medium
		}
		SAT: _all_off
		SUN: _all_off
	}
	all_medium: {
		for day in _days {
			"\(day)": _day_medium
		}
	}
	weekend_medium: {
		for day in _days
		if day != "SAT" && day != "SUN" {
			"\(day)": _all_off
		}
		SAT: _day_medium
		SUN: _day_medium
	}
	minimum: {
		for day in _days {
			"\(day)": _day_minimum
		}
	}
}

capacity_schedules: {
	for name, schedule in _base_schedules {
		"\(name)": schedule
	}
	turned_off: {
		for day in _days {
			"\(day)": _all_off
		}
	}
}
