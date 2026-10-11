# Canned Schedules

A CUE-based toolkit for defining, validating, comparing, and rendering reusable
capacity schedules. Start from a preconfigured weekly pattern, inspect its daily
rhythm visually, and choose the schedule that fits. When needed, compose a new
pattern while keeping the same validation rules, rendering logic, and exported
machine-readable data.

This project turns weekly capacity plans into reusable, inspectable data: define
site or team availability in a compact symbolic format, validate it with CUE,
render it as tables or SVGs for review, and export AWS-style cron transitions for
automation. The goal is to make capacity schedules easy to reuse and compare
without limiting more specialized configurations.

## Schedule configuration and visualization

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

The checked CUE, gomplate, jq, and yq versions are pinned in Mise. Update them there
and rerun `mise run check` when changing tool versions.

```powershell
mise run check
mise run render-time-table
mise run render-time-table -ruler --legend
mise run render-time-table -ruler --legend --offset 2
mise run render-time-table --markdown -ruler --legend --offset 2
mise run render-time-table --markdown --svg -ruler --legend --offset 2
mise run render-schedule-svg
mise run render-schedule-svg --ruler
mise run render-schedule-svg --ruler --offset 2
```

- Edit the reusable presets under `_base_schedules` in
  [schedules.cue](./schedules.cue) to create or change schedules. The
  `capacity_schedules` block in that file assembles the exported schedules.
  Generated contribution patches add explicit presets at the end of that block,
  immediately before `...`, without changing `_base_schedules`.
- `mise run check` validates schedules, checks the exported snapshot, exercises
  invalid contract cases, and smoke-tests rendering.
- Render tasks write artifacts to a temporary directory and print its path.
- Without `--markdown`, that directory contains one `time-table-<name>.txt` per
  schedule; the combined table content is also sent to stdout.
- `-r` / `--ruler` adds time annotations and the ruler.
- `-l` / `--legend` adds the ordered capacity legend.
- `-m` / `--markdown` writes a Markdown document with one schedule per section;
  it embeds inline ASCII by default. Combine with `--svg` to embed SVG files.
- `-o` / `--offset` sets non-negative indentation for text ruler and schedule rows,
  or shifts SVG timestamps left in font-character widths without moving ticks or slots.
  It defaults to `TIME_SCALE_OFFSET` when set, otherwise `0`.
- Annotation text stays at its original position; the same offset padding is
  appended to that row. An offset of `2` aligns ruler markers with HH:MM colons.

### Previewing a proposed preset

Contributors can validate and preview a schedule before opening an issue, without
editing the catalog. Use either example in [demo/](./demo/) as a starting point:

```sh
mise run vet-proposal demo/a_issue_example_visually.md
mise run preview-proposal demo/b_issue_example_cue.md
mise run preview-proposal demo/b_issue_example_cue.md --svg --target-dir previews/
```

`preview-proposal` calls `render-time-table` with Markdown, ruler, legend, and an
offset of `2`. It saves `b_issue_example_cue.generated-text.md` beside the input.
Add `--svg` to also save `b_issue_example_cue.generated-img.svg`; proposal
Markdown stays text and does not depend on that image. Names follow the input
filename stem, while the schedule name from frontmatter appears in the content.
It also saves `b_issue_example_cue.generated.patch`, which adds the preset at the
end of `capacity_schedules` in `schedules.cue`, immediately before `...`, leaving
`_base_schedules` unchanged. All generated suffixes are gitignored.
Use `--force` to replace an existing
preview, or `--target-dir` to copy the complete bundle elsewhere.

To choose the presentation yourself, use the rendering tasks:

```sh
mise run render-time-table --proposal demo/b_issue_example_cue.md --markdown --ruler --legend --offset 2
mise run render-schedule-svg --proposal demo/a_issue_example_visually.md --ruler
mise run check-proposals
```

Proposals must start with YAML frontmatter bounded by `---` lines. `name` and
exactly one of `schedule_visual` or `schedule_cue` are required. Optional metadata
is `description`, `team`, and `owner` (a list of strings); unknown fields are
rejected. Names must start with a lowercase letter and contain only lowercase
letters, digits, and underscores, and must not already exist in the catalog.

- `schedule_visual` contains seven unlabelled rows, ordered MON-SUN, with exactly
  48 tier symbols per row.
- `schedule_cue` is a MON-SUN map of CUE expression strings. It supports shared
  day helpers (`_day_maximum`, `_day_medium`, `_day_minimum`, `_all_off`), `_t0`
  through `_t3`, quoted tier-symbol strings, integer counts, `+`, `*`, and
  parentheses. For example: `MON: "_t1*10 + _t2*28 + _t1*10"`. It is not an
  unrestricted CUE program.

The task extracts frontmatter with `yq --front-matter extract`, builds an isolated
CUE configuration, validates it against the existing contracts, and renders only
the proposed preset, including derived UTC cron transitions. Successful previews
retain `proposal.json` (metadata), `proposal.cue` (candidate source), and
`schedule.json` in the reported temporary bundle. Only named preview files are
copied beside the input by default; `--target-dir` copies the whole bundle.
Nothing is automatically applied or
committed. A maintainer can review the candidate source before adding it to the
catalog.

#### Applying the contribution patch

Attach the generated patch to an issue, or apply it on a contribution branch:

```sh
git apply --check demo/b_issue_example_cue.generated.patch
git apply demo/b_issue_example_cue.generated.patch
mise run fmt
cue export . > test.cue.json
mise run check
```

The patch changes only `schedules.cue`; generation validates the combined catalog
and checks that the patch applies to the current working file. It does not apply,
stage, or commit anything. Review the change, refresh the exported snapshot, and
include any required generated documentation before opening a merge request.
If the catalog has changed since generation, re-check applicability or regenerate
the patch. `--force` never bypasses the duplicate schedule-name check.

Everything after the closing frontmatter delimiter is optional issue context and
is ignored by validation. Contributors may paste the generated Markdown preview
below it, attach the preview, or submit just the frontmatter. The optional SVG is
a separate convenience preview; text Markdown is convenient for pasting into an issue.
Validation establishes structural plausibility, not whether a schedule meets a
service's operational requirements.

### Markdown output

Use `--markdown` to generate a Markdown report with one schedule per section.
By default, each section contains its ASCII table in a fenced text block:

```powershell
mise run render-time-table --markdown -ruler --legend --offset 2 > schedules.md
```

The task also saves `SCHEDULES.md` in its temporary directory and prints that
directory on stderr.

Add `--svg` to generate one `schedule-<name>.svg` per configured schedule and
embed those instead:

```powershell
mise run render-time-table --markdown --svg -ruler --legend --offset 2 > schedules.md
```

The redirected Markdown contains relative SVG links. To keep those links valid,
use `SCHEDULES.md` from the reported temporary directory, or copy the SVG
sidecars beside the redirected Markdown file. The task's temporary directory
contains a ready-to-view `SCHEDULES.md` and all its SVGs.

To place the complete bundle in `output/`, set a target directory. `--force`
replaces matching generated filenames but leaves unrelated files untouched:

```powershell
mise run render-time-table --markdown --svg -ruler --legend --offset 2 --target-dir output/ --force
```

This creates `output/SCHEDULES.md` alongside `output/schedule-<name>.svg` files.
Without `--markdown`, it contains one `time-table-<name>.txt` per schedule. Use
`mise run render-schedule-svg` to generate only the SVG files.

Each SVG shows the weekly pattern, tier legend, and transitions with their AWS
cron expressions. Hovering a slot shows the transition that set its tier.
SVGs adapt to light and dark themes. Add `--ruler` for time labels; the SVG
legend is always included (`--legend` controls text output only).

Every render task exports the root CUE package and writes `schedule.json` and
`time-scale.txt` alongside its output in a retained temporary directory under
`$TMPDIR` (normally `/tmp`). Rendered content goes to stdout, so it can be
redirected; the artifact-directory message goes to stderr. The SVG-only task
prints a list of SVG paths to stdout, which can be redirected to a manifest.
No generated render files are written to the repository root. The check task uses
and removes its own temporary files.

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

The weekly schedule repeats in UTC, and Monday may start at a different tier
from Sunday's final slot. Such a boundary creates a transition at Monday 00:00
in the generated cron data. Consumers should use the transition list to apply
changes; the cron expressions do not independently establish an initial tier
when a consumer starts.

Cron day-of-week fields use a range for every contiguous group, such as
`MON-FRI`, `MON-SAT`, or `SAT-SUN`; separated days remain a comma list.

The plain-text timetable and SVG both show these UTC cron expressions beneath
each schedule. Schedules with no tier changes are marked accordingly.

## Create a schedule

The reusable weekly presets live under `_base_schedules` in
[schedules.cue](./schedules.cue): `standard`, `weekday_maximum`,
`weekday_medium`, `all_medium`, `weekend_medium`, and `minimum`. The exported
`capacity_schedules` is generated from those presets, so all six appear in the
rendered table. Add new presets in `_base_schedules`; do not add them to the
`capacity_schedules` comprehension. `all_off` is an additional generated schedule.

To create another preset, copy one of the entries in `_base_schedules`, give it
a new name, and edit its MON-SUN values. Each week has exactly seven day keys;
each quoted day value is 48 Unicode symbols, one per half-hour starting at
midnight. The model generates the all-off week by assigning `_all_off` to every
day in a comprehension.

### Draft a schedule with an LLM

You can describe the weekly pattern in plain language and ask an LLM to translate
it into a preset. Be explicit about the default tier and the days and times for
each override. The grid is UTC, starts at 00:00, and has 48 half-hour slots per
day, so times must fall on a 30-minute boundary.

For example, provide a request like this, replacing the schedule name and
requirements with your own:

```text
Add a reusable schedule named `office_hours` under `_base_schedules` in
schedules.cue.

The schedule repeats weekly in UTC. Each day has 48 half-hour slots starting at
00:00. The symbols are ░ Off, ▒ Minimum, ▓ Medium, and █ Maximum.

Requirements:
- Minimum every day by default.
- Medium Monday through Friday from 09:00 to 17:00.

Treat ranges as start-inclusive and end-exclusive. If a time is not on a
half-hour boundary, or the requirements leave a gap or overlap, ask me before
editing. Add only the new preset under `_base_schedules` in schedules.cue; do
not edit the `capacity_schedules` comprehension, schema, templates, or generated
files. Reuse existing schedule patterns where they fit.

After editing, run `mise run check` and `mise run render-time-table -ruler
--legend`. Summarize the schedule and report whether validation passed.
```

Review the generated table, then run `mise run check` yourself before keeping
the change. See [schedules.cue](./schedules.cue) for the existing presets and
the exact CUE structure.

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
mise run render-time-table -ruler --legend
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
mise exec -- gomplate -c .=validated-schedule.json -t time-scale=time-scale.tpl -f time-table.tpl
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

## Example output

![standard capacity schedule](output/schedule-standard.svg)
