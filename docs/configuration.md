# Configuration

The built-in configuration ([`schema/config.json`](https://github.com/iilei/canned-schedules/blob/master/schema/config.json), validated by
[`schema/config.schema.json`](https://github.com/iilei/canned-schedules/blob/master/schema/config.schema.json)) defines the ordered state list and how states look: page
colours per light/dark theme, one style (symbol and colours) per state assigned round robin, and the symbol preset of
the ASCII preview (`representation.ascii.symbols`). Override any part of
it per run:

```sh
./canned-schedules \
  --set-config-at 'states[0].display-name' --set-config-value '"Switched off"' \
  schedule.md
```

An override replaces the value at its path; the path and the array index must exist.

## Your own states

Replace the whole `states` list to schedule something other than capacity. For example, follow-the-sun on-call:

```sh
./canned-schedules \
  --set-config-at states \
  --set-config-value '[{"id": "Nobody",  "display-name": "Nobody"},
                       {"id": "Team-EU", "display-name": "Team EU"},
                       {"id": "Team-US", "display-name": "Team US"}]' \
  oncall.md
```

```markdown
# On-Call

> [!SCHEDULE-BASE] Nobody

> [!SCHEDULE-STATE] Team-EU
> - `06:00` to `14:00` MON-FRI

> [!SCHEDULE-STATE] Team-US
> - `14:00` to `22:00` MON-FRI
```

- Only the listed ids are valid in BASE and STATE blocks. The first one is the base when the document has none.
- Colours and symbols are assigned round robin from `representation.styles.states`, so override those only for
  specific colours.
- To share a configuration, keep it in a file and pass `--set-config-value "$(cat states.json)"`.
- The week repeats unchanged. Rotations that alternate between weeks ("Alice this week, Bob next week") can't be
  expressed, and overlapping roles such as primary and secondary on-call need one document each.
