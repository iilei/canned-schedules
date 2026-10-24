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
valid/<case>/   input.md, want.json, [args]
warn/<case>/    input.md, want.json, want.diag, [args]
error/<case>/   input.md, want.diag, [args]
```

- `input.md`: the document under test.
- `args`: CLI arguments, **one per line** (no shell quoting). Omitted means defaults: built-in config,
  `--granularity 1`, `--require-base false`, no `--base`.
- `want.json`: the expected canonical output (draft shape, see below). Error cases have none, because errors produce
  no output.
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
- an alert marker: the first character after `[!` (`SCHEDULE:TEIR` → the `S`);
- a malformed range item: the first character of the list item's content (after `- `).

## Canonical output (draft v1)

```json
{
  "schema-version": "1",
  "name": "Schedule-Name",
  "hint": "Office hours only",
  "base": "Minimum",
  "annotations": {"x-custom-owner": "team-xyz"},
  "week": {
    "MON": [{"start": "00:00", "end": "06:00", "tier": "Minimum"}, …],
    "TUE": […], "WED": […], "THU": […], "FRI": […], "SAT": […], "SUN": […]
  }
}
```

- `week` keys are always all seven days, in order `MON`…`SUN`.
- Each day is split at midnight into runs that cover `00:00`–`00:00` without gaps. Runs are half-open. An `end` of
  `00:00` means the end of that day. Adjacent runs with the same tier are merged.
- Tier names use the **config's spelling** of the id, whatever case the document used.
- Times are always `HH:MM` in the output, even when the input used `H:MM` (`6:00` → `06:00`).
- `name` comes from the H1. `hint` is the text after the META marker and is **omitted** when there is none.
- `annotations` holds every `x-custom-*` META key verbatim (`{}` if none).
- Not yet included: the request and effective digests, and derived transitions. They get added once the output schema
  exists, and the fixtures will then be extended.

## Built-in config assumed

The fixtures assume the tiers from `schema/config.json`: `Off`, `Minimum`, `Medium`, `Maximum` (index 0 = `Off`), four
styles. Namespace aliases are fixed in code: `SCHEDULE`, `CANNED-SCHEDULE`, `TO-SCHEDULE`,
`CAN-SCHED`, `CONFIG-SCHED`.

## Notes

- `valid/weekday-whitespace` has **trailing spaces on purpose** (after `SAT, SUN`). Don't let editors strip them.
- Diagnostic codes are a draft vocabulary: `invalid-time`, `invalid-weekday`, `invalid-range`, `unknown-kind`,
  `unknown-tier`, `missing-title`, `missing-base`, `meta-schema`, `granularity`, `invalid-flag`, `invalid-override`,
  `config-schema`, `config-duplicate-tier`, `config-duplicate-symbol`, `unknown-namespace`, `style-reuse`, `missing-kind`, `duplicate-block`,
  `empty-tier`, `misplaced-directive`, `unexpected-content`, `missing-name`, `duplicate-name`, `invalid-name`, `invalid-hint`.

## Undecided: no fixtures yet

These cases need a spec decision first. See "Open (fixtures)" in `CLAUDE.md`.

- None at the moment.
