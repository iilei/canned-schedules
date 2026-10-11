---
name: nightly_active
description: Medium capacity every night from 22:00 to 06:00 UTC, Off otherwise.
schedule_cue:
  "MON": "_t2*12 + _t0*32 + _t2*4"
  "TUE": "_t2*12 + _t0*32 + _t2*4"
  "WED": "_t2*12 + _t0*32 + _t2*4"
  "THU": "_t2*12 + _t0*32 + _t2*4"
  "FRI": "_t2*12 + _t0*32 + _t2*4"
  "SAT": "_t2*12 + _t0*32 + _t2*4"
  "SUN": "_t2*12 + _t0*32 + _t2*4"
---

# Nightly active capacity

The same pattern repeats every day, including weekends:

- 00:00–06:00 UTC: Medium.
- 06:00–22:00 UTC: Off.
- 22:00–24:00 UTC: Medium, continuing into the next morning.
