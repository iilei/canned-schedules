# Schedule configuration and visualization

The goal is to make capacity schedules easy to reuse and compare without limiting
more specialized configurations. Start with a preconfigured schedule, inspect its
daily pattern visually, and select what fits. When necessary, compose a different
pattern while retaining the same validation rules.

[test.cue](./test.cue) contains the example configuration and derived display
data. [schedule-schema.cue](./schedule-schema.cue) defines the reusable weekly
schedule contract. CUE validates and exports; gomplate only renders the resulting
JSON.

## Quick start

With mise and the tools configured in [mise.toml](./mise.toml) available:

The checked CUE, gomplate, and jq versions are pinned in Mise. Update them there
and rerun `mise run check` when changing tool versions.

```powershell
mise run check
mise run render-time-table
mise run render-time-table -r -l
mise run render-time-table -r -l --offset 2
```

- `mise run check` validates schedules, checks the exported snapshot, exercises
  invalid contract cases, and smoke-tests rendering.
- Each configured stage appears as a separate section, with MON-SUN rows in order.
- `-r` / `--ruler` adds time annotations and the ruler.
- `-l` / `--legend` adds the ordered capacity legend.
- `-o` / `--offset` sets non-negative indentation for the ruler and schedule rows.
  It defaults to `TIME_SCALE_OFFSET` when set, otherwise `0`.
- Annotation text stays at its original position; the same offset padding is
  appended to that row. An offset of `2` aligns ruler markers with HH:MM colons.

The table task reuses `render-time-scale`, passing through the offset. That task
exports the root CUE package to `schedule.json` and generates `time-scale.txt`;
the table itself is printed to standard output. The check task uses temporary
files instead of writing these artifacts into the project directory.

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
separate root-level CUE file, use `package schedules`; the Mise tasks load the
root package, including [schedule-schema.cue](./schedule-schema.cue).

**CUE combines constraints; it does not use last-write-wins overrides.**
For example, `#base & {SUN: _day_Medium}` conflicts with the base's concrete
all-off Sunday. Design reusable definitions with configurable fields when more
variation is needed, or compose only the unchanged days as above.

## JSON and CUE should share the validation boundary

JSON is useful for consumers that prefer explicit values; CUE is useful for
authors who want composition and reuse. Both should be checked against the same
CUE constraints before rendering or downstream use.

The shared weekly structure and legend contract can validate an independent JSON
input:

```powershell
mise exec -- cue vet -d '#schedule_contract' schedule-schema.cue input.json
```

This checks the weekly keys, daily slot lengths, legend symbols and tiers, and
that schedule symbols are declared. The full preset's tier value relationships
remain in [test.cue](./test.cue). For that concrete configuration, use:

```powershell
mise exec -- cue vet .
mise exec -- cue export . --outfile validated-schedule.json
```

Combining additional JSON with the root package also intersects it with the
concrete `int` preset; it does not replace that preset. The schema-only check
does not derive the time ruler or annotations, so it is not yet a general
render-ready JSON input mode.

Render that validated JSON directly:

```powershell
mise exec -- gomplate -c .=validated-schedule.json -t time-scale=time-scale.tmpl -f time-table.tmpl
```

This direct invocation shows schedule rows only. The current Mise rendering tasks
always regenerate `schedule.json` from the root CUE package; they do not accept an
external JSON input. Sending JSON directly to gomplate bypasses CUE validation.

## Current sanity checks and their boundaries

The current CUE configuration enforces:

- Exactly the seven weekday keys MON through SUN for every schedule.
- Exactly `_slots` Unicode runes for every daily value under every stage in
  `capacity_schedules`, including newly added stages.
- Every schedule symbol appears in the legend; legend symbols are single-rune
  and unique, and each legend tier exists in `capacity_tiers`.
- Exactly `_slots` time brackets, with integer minutes in `[0, 1440)`.
- Exactly `_slots` runes in the generated ruler.
- The constraints written on `min`, `desired`, and `max` in each capacity tier.

Use:

```powershell
mise run check
mise exec -- cue vet .
mise exec -- cue export .
```

These checks apply to the final composed values, not just to how they were
authored. Changing the slot count also requires updating any fixed-length preset
patterns; validation exposes inconsistencies instead of silently resampling them.

The model does not establish a timezone/DST policy or general min/desired/max
relationships for newly added tier names. Those policies remain open design
choices; the shared schema currently validates tier shape and legend references.

Gomplate validates the presentation offset separately: empty, negative,
non-integer, and overflowing offsets fail explicitly. Presentation settings do
not change schedule capacity values.
