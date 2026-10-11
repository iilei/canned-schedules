# Schedule configuration and visualization

The goal is to make capacity schedules easy to reuse and compare without limiting
more specialized configurations. Start with a preconfigured schedule, inspect its
daily pattern visually, and select what fits. When necessary, compose a different
pattern while retaining the same validation rules.

[schedules.cue](./schedules.cue) is the authoring file: add a stage and set its
MON-SUN day strings there. The model, shared tiers, legend, and derived display
data stay in [schedule-core.cue](./schedule-core.cue) and
[schedule-schema.cue](./schedule-schema.cue).
CUE validates and exports; gomplate only renders the resulting JSON.

## Quick start

With mise and the tools configured in [mise.toml](./mise.toml) available:

The checked CUE, gomplate, and jq versions are pinned in Mise. Update them there
and rerun `mise run check` when changing tool versions.

```powershell
mise run check
mise run render-time-table
mise run render-time-table -r -l
mise run render-time-table -r -l --offset 2
mise run render-time-table --markdown -r -l --offset 2
mise run render-schedule-svg
```

- Edit [schedules.cue](./schedules.cue) to create or change schedules.
- `mise run check` validates schedules, checks the exported snapshot, exercises
  invalid contract cases, and smoke-tests rendering.
- Each configured stage appears as a separate section, with MON-SUN rows in order.
- `-r` / `--ruler` adds time annotations and the ruler.
- `-l` / `--legend` adds the ordered capacity legend.
- `-m` / `--markdown` writes a Markdown document with a preset index and an
  embedded `schedule.svg` image. It can be combined with ruler, legend, and offset.
- `-o` / `--offset` sets non-negative indentation for the ruler and schedule rows.
  It defaults to `TIME_SCALE_OFFSET` when set, otherwise `0`.
- Annotation text stays at its original position; the same offset padding is
  appended to that row. An offset of `2` aligns ruler markers with HH:MM colons.

### Markdown output

Use `--markdown` to generate a Markdown report with a schedule index and an SVG
image of every configured schedule. The task writes `schedule.svg` beside the
Markdown output; redirect standard output to save the document:

```powershell
mise run render-time-table --markdown -r -l --offset 2 > schedules.md
```

The SVG tier colors come from `capacity_tiers` in the exported JSON, and its width
is derived from `time_brackets`. Use `mise run render-schedule-svg` to generate
only the SVG asset. Ruler, legend, and offset remain available in plain-text mode.

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

## AWS cron transitions

The root CUE export includes `aws_cron` data derived from each weekly schedule.
Its timezone is fixed to UTC. Each schedule has a list of tier changes with the
source tier, destination tier, affected weekdays, and an AWS six-field cron
expression (`minute hour day-of-month month day-of-week year`). For example,
`weekday_maximum` changes to Medium at `0 5 ? * MON-FRI *`. `[standard]` also has
the same Saturday transition, so its expression includes Saturday.

The weekly pattern is cyclic: Monday's first slot must use the same symbol as
Sunday's final slot. The shared CUE contract rejects schedules that break this
boundary. Consumers should use the transition list to apply changes; the cron
expressions do not independently establish an initial tier when a consumer starts.

## Create a schedule

The six reusable weekly presets live under `_base_schedules` in
[schedules.cue](./schedules.cue): `standard`, `weekday_maximum`,
`weekday_medium`, `all_medium`, `weekend_medium`, and `minimum`. The exported
`capacity_schedules` is generated from those presets, so all six appear in the
rendered table. `all_off` is an additional generated schedule.

To create another preset, copy one of the entries in `_base_schedules`, give it
a new name, and edit its MON-SUN values. Each week has exactly seven day keys;
each quoted day value is 48 Unicode symbols, one per half-hour starting at
midnight. The model generates the all-off week by assigning `_all_off` to every
day in a comprehension.

Use only symbols from the shared legend:

| Symbol | Tier |
| ------ | ---- |
| ░ | Off |
| ▒ | Minimum |
| ▓ | Medium |
| █ | Maximum |

Then validate and preview:

```powershell
mise run check
mise run render-time-table -r -l
```

The check rejects missing or unknown days, values with the wrong number of
symbols, and symbols that are not in the legend. Tier values and symbol meanings
are shared defaults; change them only in the model files.

Use the shared pattern definitions such as `#base`, `_day_medium`, and
`_all_off` when building reusable schedules. Authors can keep to literal day
strings if they prefer; the weekly and symbol constraints are checked either way.

Keep the file in the root `schedules` package. Root-level CUE files are combined
and validated together; CUE constraints do not work as last-write-wins overrides.

## JSON and CUE should share the validation boundary

JSON is useful for consumers that prefer explicit values; CUE is useful for
authors who want composition and reuse. Both should be checked against the same
CUE constraints before rendering or downstream use.

The shared weekly structure and legend contract can also validate an independent
JSON input:

```powershell
mise exec -- cue vet -d '#schedule_contract' schedule-schema.cue input.json
```

This checks the weekly keys, daily slot lengths, legend symbols and tiers, and
that schedule symbols are declared. The full root package, including
[schedules.cue](./schedules.cue), can be checked and exported with:

```powershell
mise exec -- cue vet .
mise exec -- cue export . --outfile validated-schedule.json
```

Combining additional JSON with the root package intersects it with the schedules
already defined there; it does not replace them. The schema-only check does not
derive the time ruler or annotations, so it is not yet a general render-ready
JSON input mode.

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
