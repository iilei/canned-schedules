# Syntax

A schedule is an H1 heading (the name) plus directive blocks: quotes that start with a `[!SCHEDULE-…]` marker.
Everything else is free text for humans, including standard alerts such as `[!NOTE]`.

| Block                      | Required                         | Content                                                                                               |
| -------------------------- | -------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `[!SCHEDULE-META] summary` | no                               | Optional summary after the marker; list items `` x-<name>: `value` `` for your own bookkeeping        |
| `[!SCHEDULE-BASE] state`   | no, unless `--require-base true` | The state wherever no range applies. Without the block: `--base`, else the first configured state     |
| `[!SCHEDULE-STATE] state`  | no                               | A list of ranges in that state. Any number of blocks; a state may appear in several, later ranges win |

`SCHEDULE` is the only namespace, joined to the kind with a hyphen. Markers and state names are case-insensitive. Any
other alert, including a misspelt `[!SCHEDUEL-STATE]`, is free text unless `--error-unless-alert-pattern` restricts
which alerts may appear: with `--error-unless-alert-pattern '(?i)NOTE|TIP|IMPORTANT|WARNING|CAUTION'`, any other alert
is an error. The pattern is Go regular expression syntax, matched against the whole alert type; schedule directives
are always allowed.

**Ranges** are written `` `start` to `end` WEEKDAYS ``:

- Times are `H:MM` or `HH:MM`, e.g. `6:15` or `21:45`.
- The end is exclusive: `06:00` to `22:00` covers 06:00 up to, but not including, 22:00.
- A range that runs until midnight ends at `00:00`. There is no `24:00`.
- A range belongs to its start day and may cross midnight: `22:00` to `02:00` FRI runs into Saturday 02:00.
- Equal start and end means a full 24 hours: `08:00` to `08:00` MON runs until Tuesday 08:00.
- Weekdays are `MON TUE WED THU FRI SAT SUN` in uppercase, with ranges (`MON-FRI`, also across the weekend: `FRI-MON`)
  and lists (`SAT,SUN`). Weekday ranges are **inclusive**: `MON-FRI` includes Friday, unlike the time range, whose end
  is exclusive.
- Each weekday is written at most once per range: `MON,MON` and `WED-WED` are errors (write `WED`). A day that an
  earlier range already covers, as in `MON-WED,TUE`, gets a warning: it's harmless but usually a typo.
- Later ranges win where they overlap earlier ones.
- A no-break space (as pasted from word processors) counts as an ordinary space everywhere spaces are allowed. Other
  invisible characters, such as a zero-width space, are errors and are shown escaped (`\u200b`) in messages.

The states come from the configuration: `Off`, `Minimum`, `Medium` and `Maximum` by default. A state is anything that
holds for a stretch of the week, such as a capacity level, a support team or a maintenance mode. To use your own, see
[Your own states](configuration.md#your-own-states).
