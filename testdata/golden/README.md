# Golden fixtures

These fixtures are the executable form of the spec in `CLAUDE.md`. They were written before any Go code, so when the
implementation and a fixture disagree, check the spec first. Don't regenerate the fixture to match the code.

## Generating

`gen.py` writes every case: run `python3 testdata/golden/gen.py` from the repo root and review the diff. It replaces
`valid/`, `warn/` and `error/` completely. Expected outputs come from a small reference painter inside the script,
never from the Go code.

## Layout

One directory per case, grouped by expected outcome:

```text
valid/<case>/   input.md, want.json, [args], [want.preview]
warn/<case>/    input.md, want.json, want.diag, [args]
error/<case>/   input.md, want.diag, [args]
```

- `input.md`: the document under test.
- `args`: CLI arguments, **one per line** (no shell quoting). Omitted means defaults: built-in config,
  `--sample-interval 1`, `--require-base false`, no `--base`.
- `want.json`: the expected canonical output (draft shape, see below). Error cases have none, because errors produce
  no output.
- `want.preview`: the expected day rows of the ASCII preview (`MON` + one symbol per bucket, built-in symbols
  `░▒▓█`). It pins the bucketing rule (majority, ties to the later state), not the rest of the layout.
- `want.diag`: the expected diagnostics, one per line, in emission order:

  ```text
  <severity> <line>:<col> <code>
  <severity> - <code>            # no source position (flags, config)
  ```

  Severity is `error` or `warn`. Line and column are 1-based, and columns count runes. Fixtures pin **severity,
  position and code only**. Message text is free to change.

## Position convention

A diagnostic points at the first character of the offending token:

- a value in a code span: the first character *inside* the backticks (`24:00` → the `2`);
- a weekday expression: the first character of the bad token;
- an alert marker: the first character after `[!` (`SCHEDULE-TEIR` → the `S`);
- a malformed range item: the first character of the list item's content (after `- `).

## Canonical output (draft v1)

```json
{
  "schema-version": "1",
  "name": "Schedule-Name",
  "hint": "Office hours only",
  "base": "Minimum",
  "annotations": {"x-owner": "team-xyz"},
  "digests": {"schedule": "sha256:10ab…"},
  "week": {
    "MON": [{"start": "00:00", "end": "06:00", "state": "Minimum"}, …],
    "TUE": […], "WED": […], "THU": […], "FRI": […], "SAT": […], "SUN": […]
  },
  "transitions": [
    {
      "time": "06:00",
      "days": ["MON", "TUE", "WED", "THU", "FRI"],
      "state": "Medium",
      "cron": {"posix": "0 6 * * 1-5", "aws": "0 6 ? * MON-FRI *"}
    }
  ]
}
```

- `week` keys are always all seven days, in order `MON`…`SUN`.
- Each day is split at midnight into runs that cover `00:00`–`00:00` without gaps. Runs are half-open. An `end` of
  `00:00` means the end of that day. Adjacent runs with the same state are merged.
- State names use the **config's spelling** of the id, whatever case the document used.
- Times are always `HH:MM` in the output, even when the input used `H:MM` (`6:00` → `06:00`).
- `name` comes from the H1. `hint` is the text after the META marker and is **omitted** when there is none.
- `annotations` holds every `x-*` META key verbatim (`{}` if none).
- `transitions` lists every moment the active state changes. The week is cyclic, so MON 00:00 follows SUN 23:59.
  - Days that switch to the same state at the same time are grouped into one entry. `days` is the full list, Monday
    first.
  - Order: by the group's first minute in the week (MON 00:00 first), then by state id.
  - A week painted in a single state has no transitions (`[]`).
  - `time`, `days` and `state` are the facts. `cron` renders them for schedulers, without a time zone (wall-clock):
    - `posix`: five-field Unix cron with **numeric** weekdays, 0 = SUN (as EC2 Auto Scaling scheduled actions use).
      Classic cron doesn't allow ranges of names.
    - `aws`: six-field EventBridge format (`min hour ? month weekday year`) with weekday **names**. AWS numbers run
      1 = SUN, so names avoid off-by-one mistakes. This is not Quartz, which has a leading seconds field.
    - Both list days Sunday-first and never wrap around the week (`FRI`–`MON` becomes `0-1,5-6` /
      `SUN-MON,FRI-SAT`). All seven days become `*`.
- `base` is always present, also when `transitions` is `[]`. It is the resolved base state (document BASE, then
  `--base`, then config state 0).
- `warnings` lists every warning (`code`, `message`, and `line`/`column`/`excerpt` when positioned). Fixtures don't pin
  message text: they hold `"…"`, and the golden runner replaces actual messages with it before comparing.
- `digests`, written `sha256:<64 lowercase hex>`:
  - `schedule`: SHA-256 of `{"base": …, "week": …}` as compact JSON (no whitespace, keys in the order shown here).
    It changes only when what a consumer acts on changes, never for name, hint, annotations or cron rendering.
- The whole output is described by `schema/output.schema.json`. The tool validates its own output against it, and a
  test validates every `want.json` here against it.

## Built-in config assumed

The fixtures assume the states from `schema/config.json`: `Off`, `Minimum`, `Medium`, `Maximum` (index 0 = `Off`), four
styles. The namespace is fixed in code: `SCHEDULE` only.

## Notes

- `valid/weekday-whitespace` has **trailing spaces on purpose** (after `SAT, SUN`). Don't let editors strip them.
- Diagnostic codes are a draft vocabulary: `invalid-time`, `invalid-weekday`, `invalid-range`, `unknown-kind`,
  `unknown-state`, `missing-title`, `missing-base`, `meta-schema`, `sample-interval`, `invalid-flag`, `invalid-override`,
  `config-schema`, `config-duplicate-state`, `config-duplicate-symbol` (a warning), `invalid-marker`, `disallowed-alert`,
  `style-reuse`, `preview-approximate`, `duplicate-weekday`, `repeated-weekday` (a warning), `missing-kind`, `duplicate-block`,
  `empty-state`, `misplaced-directive`, `unexpected-content`, `missing-name`, `duplicate-name`, `invalid-name`, `invalid-hint`.

## Undecided: no fixtures yet

These cases need a spec decision first. See "Open (fixtures)" in `CLAUDE.md`.

- None at the moment.
