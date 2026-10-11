# Schedule configuration and visualization

The goal is to make capacity schedules easy to reuse and compare without limiting
more specialized configurations. Start with a preconfigured schedule, inspect its
daily pattern visually, and select what fits. When necessary, compose a different
pattern while retaining the same validation rules.

[test.cue](./test.cue) currently combines reusable building blocks, validation
constraints, example configuration, and derived display data. CUE is responsible
for configuration and validation; gomplate only renders the resulting JSON.

## Quick start

With mise and the tools configured in [mise.toml](./mise.toml) available:

```powershell
mise run render-time-table
mise run render-time-table -r -l
mise run render-time-table -r -l --offset 2
```

- Each configured stage appears as a separate section, with MON-SUN rows in order.
- `-r` / `--ruler` adds time annotations and the ruler.
- `-l` / `--legend` adds the ordered capacity legend.
- `-o` / `--offset` sets non-negative indentation for the ruler and schedule rows.
  It defaults to `TIME_SCALE_OFFSET` when set, otherwise `0`.
- Annotation text stays at its original position; the same offset padding is
  appended to that row. An offset of `2` aligns ruler markers with HH:MM colons.

The table task reuses `render-time-scale`, passing through the offset. That task
exports CUE to `schedule.json` and generates `time-scale.txt`; the table itself is
printed to standard output.

The scale can also be rendered independently:

```powershell
mise run render-time-scale --offset 2
```

Use a monospace font and preserve whitespace. The output is text art using the
configured Unicode symbols, rather than strictly ASCII.

## Reading a schedule

The current configuration divides a day into 48 half-hour slots. Every character
in a daily schedule represents one slot, starting at midnight:

```text
    00:00       06:00       12:00       18:00
    ║     :     |     :     ║     :     |     :
MON ▒▒▒▒▒▒▒▒▒▒▓▓▓▓████████████████████▓▓▓▓▒▒▒▒▒▒▒▒▒▒
```

The ordered `capacity_legend` list maps symbols to named tiers:

| Symbol | Tier |
| ------ | ---- |
| ░ | Off |
| ▒ | Minimum |
| ▓ | Medium |
| █ | Maximum |

`capacity_tiers` defines each tier's `min`, `desired`, and `max` values.
The time brackets, HH:MM annotations, and ruler are derived from the slot count.
Rendering multiple stages together makes their daily and weekly patterns easy to
compare. The renderer is an inspection tool, not an interactive schedule selector:
choosing a schedule means referencing or editing the configuration.

## Reuse first, specialize when necessary

CUE can describe values with expressions, reusable definitions, and comprehensions
instead of requiring every daily string to be handwritten.

The existing building blocks include:

- `_day_maximum`: a composed daily capacity pattern.
- `_day_Medium`: a different daily pattern.
- `_all_off`: the off symbol repeated for the whole day.
- `#base`: a reusable weekly schedule definition.

For example, add another stage inside `capacity_schedules`:

```cue
capacity_schedules: {
    prod: #base
}
```

That stage gets the existing base week and the map-wide daily length constraints.
It appears alongside `int` when rendered.

To specialize one day, construct a new week rather than trying to overwrite a
concrete value in the base:

```cue
capacity_schedules: {
    weekend_enabled: {
        for day in _days
        if day != "SUN" {
            "\(day)": #base[day]
        }
        SUN: _day_Medium
    }
}
```

These snippets can be incorporated into [test.cue](./test.cue). If put in a
separate CUE file, use `package schedules` and explicitly include that file in
validation/export commands; the current mise tasks export only `test.cue`.

**CUE combines constraints; it does not use last-write-wins overrides.**
For example, `#base & {SUN: _day_Medium}` conflicts with the base's concrete
all-off Sunday. Design reusable definitions with configurable fields when more
variation is needed, or compose only the unchanged days as above.

## JSON and CUE should share the validation boundary

JSON is useful for consumers that prefer explicit values; CUE is useful for
authors who want composition and reuse. Both should be checked against the same
CUE constraints before rendering or downstream use.

For the current combined configuration, a JSON input can be checked and exported
together with the CUE file:

```powershell
mise exec -- cue vet test.cue input.json
mise exec -- cue export test.cue input.json --outfile validated-schedule.json
```

Run the export only if validation succeeds. Export also fails when constraints
conflict or required output values are incomplete.

Render that validated JSON directly:

```powershell
mise exec -- gomplate -c .=validated-schedule.json -t time-scale=time-scale.tmpl -f time-table.tmpl
```

This direct invocation shows schedule rows only. The current mise tasks always
regenerate `schedule.json` from `test.cue`; they do not accept an external JSON
input. Sending JSON directly to gomplate bypasses CUE validation.

Because `test.cue` includes concrete example values, combining it with JSON also
requires agreement with those values. It is **not** currently a generic schema
that lets JSON replace the configured `int` schedule or tier values.

For a general JSON-or-CUE configuration interface, separate reusable schema
constraints from presets and concrete instance data. Validate either input format
against that schema, then export the validated instance to the same rendering
format. That separation is the intended direction, not an implemented input mode.

## Current sanity checks and their boundaries

The current CUE configuration enforces:

- Exactly `_slots` Unicode runes for every daily value under every stage in
  `capacity_schedules`, including newly added stages.
- Exactly `_slots` time brackets, with integer minutes in `[0, 1440)`.
- Exactly `_slots` runes in the generated ruler.
- The constraints written on `min`, `desired`, and `max` in each capacity tier.

Use:

```powershell
mise exec -- cue vet test.cue
mise exec -- cue export test.cue
```

These checks apply to the final composed values, not just to how they were
authored. Changing the slot count also requires updating any fixed-length preset
patterns; validation exposes inconsistencies instead of silently resampling them.

The current model does **not** yet enforce a complete seven-day week, restrict day
names, or ensure every schedule character appears in the legend. It also does not
establish a timezone/DST policy or a general tier schema for new tier names.
Those rules should be added at the shared CUE validation boundary if required,
so they apply equally to plain JSON and composed CUE configurations.

Gomplate validates the presentation offset separately: empty, negative,
non-integer, and overflowing offsets fail explicitly. Presentation settings do
not change schedule capacity values.
