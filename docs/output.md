# Output

```json
{
  "schema-version": "2026-11-18",
  "name": "Office Hours",
  "summary": "Weekdays only",
  "annotations": {"x-owner": "team-xyz"},
  "constraints": {
    "states": ["Off", "Minimum", "Medium", "Maximum"],
    "sample-interval": 1,
    "require-base": false,
    "default-base": "Off",
    "alert-pattern": null
  },
  "base": "Off",
  "week": {
    "MON": [
      {"start": "00:00", "end": "08:00", "state": "Off"},
      {"start": "08:00", "end": "18:00", "state": "Medium"},
      {"start": "18:00", "end": "00:00", "state": "Off"}
    ],
    "TUE": ["…"]
  },
  "transitions": [
    {
      "time": "08:00",
      "days": ["MON", "TUE", "WED", "THU", "FRI"],
      "state": "Medium",
      "cron": {"posix": "0 8 * * 1-5", "aws": "0 8 ? * MON-FRI *"}
    }
  ],
  "occurrences": [
    {"dow": "MON", "time": "08:00", "state": "Medium", "cron": {"posix": "0 8 * * 1", "aws": "0 8 ? * MON *"}},
    "…"
  ],
  "warnings": []
}
```

- `constraints` records the rules the document was checked against, as the configuration and the flags set them:
  `states` it may use, the `sample-interval` in minutes, whether a BASE block is required (`require-base`), the state used
  without one (`default-base`, `null` when it is required) and the `alert-pattern` other alerts must match (`null` allows
  any). Consumers must accept every `sample-interval` the schema lists: the divisors of 60 and full hours up to 1440.
- `week` covers every day from `00:00` to `00:00` in half-open runs; an `end` of `00:00` means the end of the day.
- `transitions` lists every moment the state changes, grouped by time and state, so time and state together identify
  a transition within a schedule (there is no id field; see the AWS recipes for naming). `cron.posix` is five-field Unix cron
  (as EC2 Auto Scaling scheduled actions use), `cron.aws` the six-field format of EventBridge and Application Auto
  Scaling. Neither carries a time
  zone: set it where you schedule them.
- `occurrences` lists the same moments one weekday at a time, in the order of the week, each with a cron for that day only.
  Use it when each day needs its own value, such as who comes before and after a shift: the week repeats, so the first entry
  follows the last.
- `warnings` lists warnings (e.g. `style-reuse`, `preview-approximate`) with code, message and, when positioned,
  line, column and excerpt. No file names: the output depends only on the document and the flags.
- **No id, hash or digest:** `base` and `week` are what a consumer acts on, and they are canonical (fixed day order,
  merged runs, `HH:MM`), so name, summary, annotations and prose never change them. To detect a change, compare
  them, or hash them yourself, e.g. `jq -c '{base, week}' schedule.json | sha256sum`. Any identifier you need (an id,
  a hash, a git SHA, an issue link, a nonce) belongs in an `x-*` META key such as `x-id`; it passes through to
  `annotations` unchanged, and its meaning is yours.
- The schema is [`schema/output.schema.json`](https://github.com/iilei/canned-schedules/blob/master/schema/output.schema.json); details in
  [`testdata/golden/README.md`](https://github.com/iilei/canned-schedules/blob/master/testdata/golden/README.md).

Provider settings such as capacity numbers are out of scope: attach them to the state ids downstream.
