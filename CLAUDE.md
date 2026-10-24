# Agent steering — canned-schedules (work in progress)

This repo is mid-migration. Many files are leftovers from earlier designs. Read this before changing anything.

**All existing CUE, jq, Markdown/AST and shell code, up to and including commit `3ec7494`, is work-in-progress churn**
(decided 2026-10-24). It sets no contract, needs no backward compatibility and is not a reference oracle. The
decisions and the syntax below are the spec. Old code is at most a source of ideas or test cases.
Update the "Decisions" and "Open" sections when an item is resolved or a new one appears.

## Goal

Contributors propose weekly capacity schedules through a GitHub/GitLab **issue template** written in simple Markdown.
The Markdown is parsed, validated and rendered as a preview (ASCII/SVG). The core model has to work with any cloud
provider. The first target is AWS ASG scheduled actions. **Readability for humans comes first**: the input contract
should read like prose, with backticks marking only the values a machine has to read precisely.

## How the design evolved (newest last)

1. **CUE catalog (churn).** `schedule-schema.cue`, `schedule-core.cue` and `schedules.cue` hold schedules as 48-char
   slot strings per weekday plus tiers, legend and theme. `render.sh`, `proposal.sh` and `*.tpl` render tables and
   SVGs from it. Tasks: `check`, `vet-proposal`, `preview-proposal`, `render-*`.
2. **Range-based Markdown (`7d9edcb`, churn).** One `## Tier` heading per tier, then `**MON-FRI**`, then a list of
   ranges. Leftovers: `TODO.md`, `commonmark/extract_times.jq`, `commonmark/tests/task-list.md*`,
   `commonmark/ast.jsonschema.json`, tasks `markdown-extract-times` and `validate-schema`.
3. **Admonition transitions (`3ec7494`, churn).** Spec `plan.md`, sample `commonmark/lenient-expressions.md`,
   pipeline `mise run md:extract` (markdown-tool → jq filters + `schedule.pattern` → yq frontmatter).
4. **Target design (2026-10-24, not implemented yet):** a single Go binary, the namespaced `[!SCHEDULE:*]` syntax with
   ranges, a painting evaluator, and a JSON config validated by JSON Schema. Everything below.

## Decisions — architecture

- **One Go binary (decided).** No CUE (dropped 2026-10-24), no jq, yq, gomplate, `markdown-tool` or sh glue at
  runtime. `rumdl` and `mise` stay as development tools.
  - **Parsing with goldmark:** byte offsets from goldmark segments make `file:line:col` errors possible. Block nodes:
    `Lines()` segments. Code spans: the segments of their text children. Inline nodes other than text take their
    position from their first text child. Count columns in runes.
  - **Validation with JSON Schema everywhere:** the config, document META, and the tool's own generated output (as a
    safety net). Pick a maintained Go JSON Schema library (for example `santhosh-tekuri/jsonschema`; check the current
    version and draft support first).
  - **Rendering with `text/template`.** The old templates use only `add`, `sub`, `strings.Repeat`, `tmpl.Exec` and
    `test.Fail`, so they port with a small FuncMap if any are reused.
  - **Same code builds as a CLI and as WASM.** Dropping CUE keeps the WASM file small.
  - **Build stage by stage** (parse → normalise → evaluate → validate → render), with golden fixtures written from this
    spec. Remove the old mise tasks when the Go tool replaces them.
  - YAML isn't needed any more. If it comes back: `go.yaml.in/yaml/v3` parses YAML only, not frontmatter, so split it
    off yourself and use `yaml.Node` for line numbers.
- **Digests (decided, implemented).** The output carries `digests.request` (SHA-256 of the exact input bytes) and
  `digests.effective` (SHA-256 of compact `{"base", "week"}` JSON). Prose-only edits change only the request digest.
  `base` is part of the effective digest because consumers use it as the fallback, for example when `transitions` is
  `[]`. Both are written `sha256:<hex>`.
  - Still proposed: storing the raw input next to the output (`schedules/<name>/source.md` + `schedule.json`).
    Consumers read only the output; the raw Markdown is never part of the contract.
- **Linting (proposed).** rumdl has no custom rules, and a markdownlint-cli2 JS rule would be a second parser.
  - Use rumdl for generic style only. Add a per-file ignore for MD028 on schedule files, because adjacent alerts
    trigger it.
  - Add a `lint-schedule` mise task that wraps the Go validator and prints rumdl's concise format,
    `file:line:col CODE message`.

## Input syntax

No frontmatter. Everything uses one grammar: an alert in the `SCHEDULE:` namespace (or one of its fixed aliases).

```markdown
# Schedule-Name

> [!SCHEDULE:META]
> - schema-version: `1`
> - x-custom-owner: `team-xyz`

> [!SCHEDULE:BASE] Minimum

> [!SCHEDULE:TIER] Regular
> - `06:00` to `22:00` MON-FRI
> - `10:00` to `00:00` SAT,SUN

> [!SCHEDULE:TIER] Maximum
> - `22:00` to `02:00` FRI
```

### Structure (decided)

- **Namespace: a fixed allowlist, not configurable** (decided 2026-10-24). `SCHEDULE`, `CANNED-SCHEDULE`,
  `TO-SCHEDULE`, `CAN-SCHED` and `CONFIG-SCHED` are treated the same and normalised to `SCHEDULE`. They are read
  case-insensitively and may be mixed in one document. Adding an alias takes a merge request and a new release.
  - An alert type containing `:` whose namespace is not on the list is a **hard error** (`unknown-namespace`), with a
    "did you mean" hint for near-misses such as `SCHEDUEL`. There is no warn-and-ignore mode, which removes the earlier
    conflict between "unknown namespace is an error" and "ignored with a warning in custom-namespace mode".
  - Alerts without `:` (NOTE, TIP, …) are prose for humans and are ignored.
  - An unknown kind in a known namespace (`SCHEDULE:TEIR`) is a hard error (`unknown-kind`).
- **Title on the marker line** carries the name (`TIER`, `BASE`). That isn't GFM alert syntax, but it's harmless: GitHub
  and GitLab show custom alerts as literal text anyway, and our goldmark extension defines the grammar.
- **META** is a list of `key: `value`` items, in one form only. A list renders the same everywhere, while `>` lines
  without a list form one paragraph, and whether they break depends on the renderer.
  - Allowed keys: `schema-version` and `x-custom-*` (free-form). Nothing else for now.
  - **The META block and its `schema-version` are both optional.** Whenever `schema-version` is missing, whether the
    block is absent or just lacks the key, the version built into the binary (currently `"1"`) applies.
  - **Text after the META marker is the schedule's hint** (`[!SCHEDULE:META] Office hours only`). It is trimmed, then
    validated with the same rule as the name (see below), and output as `hint`. A META block with a hint and no items is
    allowed.
  - `x-custom-*` keys are allowed **only in META**. Their values are uninterpreted strings, passed through to the output
    (for example under `annotations`). They must never affect the domain model.
- **TIER items as ranges:** in each item, the times are code spans, `to` is a literal keyword that is part of the
  contract, and the weekday expression is plain text. In goldmark, an item is `Code, Text(" to "), Code, Text(days)`.
  Only `to` is accepted (no `-`, `–` or `until`).

### Time and weekday rules (decided)

- **Ranges are half-open**, `[start, end)`: `00:00` to `06:00` means `[00:00, 06:00)`.
- **Crossing midnight:** the weekday names the start day. The part after midnight spills into the next day even if
  that day isn't listed (`22:00` to `02:00` FRI covers FRI 22:00 to SAT 02:00).
- **`24:00` is rejected.** It is the same time as `00:00`. Valid times are `00:00`–`23:59`.
- **Time format:** `H:MM` or `HH:MM` (decided 2026-10-24): `6:00` and `06:00` are both fine. Hours `0`–`23`, minutes
  always two digits (`6:0` and `006:00` are rejected). The output always uses `HH:MM`.
- **Start == end means a full 24 h** that spills into the next day: `` `08:00` to `08:00` MON `` covers MON 08:00 to
  TUE 08:00.
- **Weekdays:** English three-letter names in uppercase only: `MON TUE WED THU FRI SAT SUN`. No aliases, no lowercase
  or mixed case, no trailing dots, no other variants.
  - Ranges (`MON-FRI`) and comma lists (`SAT,SUN`) are allowed. Wrap-around ranges are allowed (`FRI-MON` = FRI, SAT,
    SUN, MON).
  - Whitespace from the first weekday name to the end of the line is ignored (`SAT, SUN` and `MON - FRI` are fine).
- **Cron forms are not accepted** as input.
- **`--sample-interval <minutes>`** (renamed from `--granularity`, 2026-10-24, for one vocabulary with
  `--preview-downsample`): default `1`. It must divide 60 evenly (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60). Every
  time must be a multiple of it, otherwise it's an error (`sample-interval`). This is an **input rule**: there is no
  rounding, ever.

### Evaluation (decided)

- **Painting in minutes:** start with every minute of the week set to BASE, then paint each range in source order,
  with later ranges winning where they overlap. Every minute gets exactly one tier. The transitions for ASG scheduled
  actions are derived from the painted week as an output, not written by the user.
- **BASE via CLI flags:**
  - `--require-base true`: the document must contain `[!SCHEDULE:BASE]`. `--base` must not be given (error).
  - `--require-base false` (**the default**): the base comes from the first of these that exists: the document's
    BASE, then `--base <tier>` (must name a valid tier), then the first tier (index 0) in the config.
- **Preview downsampling (visual only, decided 2026-10-24; supersedes the earlier GCD-based grid):** previews (SVG
  and ASCII) show buckets of N minutes. (none) or `--preview-downsample` → 15 (96 columns, `MON` + 96 = 100
  characters); `--preview-downsample=N` → N (must divide 60; the value must be attached with `=`);
  `--no-preview-downsample` → 1 (1440 columns). Combining the last two is `invalid-flag`.
  - **Never applied to the canonical JSON** (week, transitions, digests stay exact) and never to input reading.
  - A bucket shows the tier covering most of its minutes; ties go to the tier that comes later in the config. When
    buckets mix tiers, a `preview-approximate` warning says how many; it appears only when a preview is requested.
  - Anyone who needs exact previews sets `--sample-interval` to a multiple of the bucket size (e.g. 15).
- **Warnings in the canonical JSON (decided 2026-10-24):** `warnings` lists every warning (`code`, `message`, and
  `line`/`column`/`excerpt` when positioned), sorted by position. No file names, so the output doesn't depend on where
  the file lives. Not part of the effective digest. Golden fixtures don't pin warning messages (`"…"`).

### Tier names (decided)

- Pattern `^[A-Za-z][A-Za-z0-9]*(?:-[A-Za-z0-9]+)*$`, 1–32 chars, ASCII, hyphen as the only separator. Names work
  directly as MyST/CSS classes.
- Case-insensitive. Canonical key = upper-cased name. Reject configs whose tier ids collide after upper-casing. Display
  name = `display-name` from the config.
- Rejected: base64-encoding names (`markdown-tool` upper-cased them, which corrupts base64, and the result is
  unreadable).

## Config (decided)

Draft files (validated with `mise exec -- jsonschema validate`): `schema/config.schema.json`, `schema/config.json`
(defaults taken from the old CUE values), `schema/meta.schema.json`. JSON Schema draft 2020-12.

- **A repo-level JSON file with a JSON Schema next to it**, both embedded in the binary with `go:embed`. The built-in
  config is validated by **tests**, not at runtime.
- **No custom schema.** The schema is fixed. Extension happens only through `x-custom-*` keys in document META.
- **Shape:** domain and representation are kept separate.

  ```text
  schema-version            const "1"
  tiers[]                   {id, display-name}: ordered, index 0 = default base
  representation.theme      {light, dark} × {background, label}: page-level colours
  representation.ascii      {symbols}: ascii | braille | styles
  representation.styles.tiers[]
                            {symbol, color: {light, dark}}: round robin, no tier id
  ```

  - Styles carry **no tier id**, so round robin works: tier *i* gets `styles.tiers[i mod len]`. Users steer the
    mapping by ordering both lists.
  - Colours: `#RRGGBB` or `#RRGGBBAA` (`"color": {"light": "#54C6A5D9", "dark": "#9D82EAD9"}`, as in the old
    `temp.json`).
  - `symbol` is one character, used for ASCII grids and the legend.
  - Every object has `additionalProperties: false`. `x-custom-*` is not allowed in the config.
- **Checks JSON Schema can't express** (done in Go, covered by tests):
  - tier ids unique after upper-casing;
  - repeated style symbols are a **warning** (`config-duplicate-symbol`, decided 2026-10-24: permissive), because
    colours can still tell tiers apart;
  - **WARN** if `len(styles) < len(tiers)`: round robin then repeats symbols and colours, so the ASCII grid and legend
    become ambiguous.
- **No namespace in the config** any more (`parser.namespace` was removed). The aliases are a constant in the Go code.
  Each one matches `^[A-Z](?:-?[A-Z0-9])*$`: a letter first, a letter or digit last, single hyphens between.
- **Tier id pattern:** `^[A-Za-z](?:-?[A-Za-z0-9])*$`, max 32 chars. It's the same rule as before, written in the same
  style as the namespace alias rule.
- **Document META** (`schema/meta.schema.json`):
  - `schema-version` is optional, defaulting to the built-in version. It is the **string** `"1"`, because every META
    value comes from a code span.
  - Otherwise only `x-custom-<name>` keys, with `<name>` lowercase letters/digits and single hyphens, up to 40 chars.
  - At most 40 custom keys (`maxProperties: 41`). Each value is a string of at most 48 chars, so 40 × 48 = 1920 chars,
    which stays under the 2 KB budget for ASCII. JSON Schema counts code points, not bytes, so non-ASCII values can
    exceed 2 KB slightly. That was accepted in exchange for checking everything in the schema.
- **Overrides via repeatable flag pairs** `--set-config-at <path>` / `--set-config-value <json>`. File paths are not
  accepted. Users who want a file can pass `"$(cat file.json)"`.

  ```sh
  --set-config-at 'representation.styles.tiers[0]' \
  --set-config-value '{"symbol": "░", "color": {"light": "#A7DCC840", "dark": "#B9A5FF40"}}'
  ```

  - The pairs are matched by position: the n-th `-at` goes with the n-th `-value`. Unequal counts are an error.
  - The path syntax is dot-separated keys plus `[index]`.
  - The full config is validated against the embedded schema **after** all overrides are applied, when the tool is
    invoked.
- The `_` protected-key convention is **dropped**.
- **Override semantics:** replace the value at the path, never deep-merge. Pairs are applied in order. An index must
  already exist, and an unknown key in the path is an error, so a typo can't create a key. To add or remove tiers,
  replace the whole list (`--set-config-at tiers`).
- **Out of scope:** capacity numbers and any other provider-specific settings (for example ASG `min`/`desired`/`max`).
  **Tier ids are the integration edge:** downstream tooling attaches provider config to tier ids. Never add such fields
  to the config or the output.
- **`schema-version` is a string** (`"1"`) everywhere: config, META and output.
- **Schema `$id`s** point to the raw files on GitHub, for example
  `https://raw.githubusercontent.com/iilei/canned-schedules/master/schema/config.schema.json`. `config.json` references
  it through `$schema`. These are placeholders, possibly moving to GitHub Pages later. A branch URL can change under
  you, so consider tag-pinned URLs once there are releases.

### Transitions in the output (decided 2026-10-24)

- Part of the **canonical output** (`transitions`), derived from the painted week. `week` stays the primary data,
  because transitions alone don't give the tier in effect right now.
- Each entry: `time`, `days` (**full list**, Monday first), `tier`, and `cron` with two dialects: `posix` (five
  fields, numeric weekdays, 0 = SUN; EC2 Auto Scaling) and `aws` (six-field EventBridge format, weekday names). Real
  Quartz (with a seconds field) only if someone needs it.
- Grouped by (time of day, tier); ordered by first minute in the week, then tier. Cron day lists are Sunday-first and
  never wrap around the week. All seven days become `*`. No time zone in cron strings.
- Implemented in `internal/schedule/transitions.go`, with an independent reference in `testdata/golden/gen.py`.

### Document structure rules (decided 2026-10-24)

- **The H1 is mandatory** and becomes `name` in the output. There must be exactly one (`missing-name`,
  `duplicate-name`).
- **Name and hint share one text rule** (`invalid-name`, `invalid-hint`): trimmed, 1–80 chars, ASCII; starts with a
  letter; may contain letters, digits, spaces and `-`; ends with a letter or digit. Regex:
  `^[A-Za-z](?:[A-Za-z0-9 -]{0,78}[A-Za-z0-9])?$`.
- **A marker with a namespace but no kind** (`[!SCHEDULE]`, `[!SCHEDULE:]`) is an error (`missing-kind`).
- **At most one META block and one BASE block** (`duplicate-block`, reported at the second one).
- **A TIER block must contain at least one range** (`empty-tier`).
- **Directives only count at the top level.** A directive nested in a list or another container, or a bold-wrapped
  marker (`> **[!SCHEDULE:TIER]**`), is an error (`misplaced-directive`). It is never silently ignored.

## Error reporting (decided and implemented 2026-10-24)

- **Non-goal: the CLI never contributes to repos.** No posting of issue comments, no API calls. Integrations (CI
  jobs, bots) take the report and do that themselves.
- **Input:** always exactly one file after the flags (no stdin). Opened once; the *opened* file must be a regular file
  (no directories, devices, pipes); at most 256 KiB (GitHub issue bodies are capped at 65,536 characters).
- **Paths in messages and reports** (`report.DisplayPath`): as given, except that absolute paths inside the home
  directory become `~/…` (OS-specific separator), so reports don't spell out the user name. Never resolved to real
  paths: that adds no safety for a CLI running with the caller's rights, is racy (TOCTOU), breaks symlinked setups,
  and makes paths absolute, which leaks more. OS errors are reduced to their reason (`report.Reason`), because
  `*fs.PathError` texts repeat the full path. File access always uses the path as given. In CI, relative paths keep
  annotations working (runner workspaces often lie inside home). If a bot ever passes attacker-derived names,
  confinement via `os.Root` (`--root`) would be the tool; not built.
- **Flags:** `--error-output-file <path>`, `--error-output-format txt|json` (default `txt`), `--error-report-template
  markdown-gfm`. Format and template are mutually exclusive (`invalid-flag`).
- **The file is always written** when `--error-output-file` is given, even with zero diagnostics, so CI never reads a
  stale report. Without it, the report goes to stderr only when there is something to report.
- **JSON report** (`internal/report`, `schema/report.schema.json`): `schema-version`, `file`, `summary` (errors,
  warnings), `diagnostics` with `severity`, `code`, `file`, `line`, `column`, `location` (ready-made `path:line:col`;
  structured fields stay because paths may contain `:`), `message`, `excerpt` (`text` + `marker` with a caret).
  Warnings are included.
- **Template:** exactly one, `templates/markdown-gfm.tmpl` (package `templates`, embedded). Plain Go `text/template`,
  no functions beyond the builtins, executed on the report decoded from its own JSON, so it runs unchanged with
  gomplate: `gomplate -c .=report.json -f templates/markdown-gfm.tmpl` is byte-identical to the built-in output
  (verified with gomplate 5.2.0). No custom template flag; other layouts use the JSON with gomplate.
- **Output safety:**
  - The report file is written atomically (temp file in the same directory + rename, mode 0644). A planted symlink is
    replaced, not written through. Directories are refused.
  - `diag.Sanitize` escapes control characters (except tab), U+2028/U+2029 and bidi formatting characters as
    `\uXXXX` in every echoed path and message (log-line forgery, GitHub `::` workflow commands, "Trojan Source").
  - In the Markdown, every user-derived string (file, message, excerpt) is printed only inside a `~~~~~~` fence, so
    Markdown, HTML, links, `@mentions` and `#references` don't render. Source lines starting with `~~~` get no excerpt,
    so they can't close the fence.
- **Exit codes:** 0 ok, 1 errors in the document or flag values, 2 unusable command line or environment
  (`invalid-args`, `input-file`, report not writable).

## SVG preview (implemented 2026-10-24)

- `--svg-output-file <path>`: written atomically (`files.WriteFile`), **only when the document has no errors**, so a
  failed run leaves an earlier preview untouched.
- Same approach as the old gomplate renderer and the error report: a repo-baked template (`templates/schedule-svg.tmpl`,
  embedded) rendering a JSON data shape. **Layout arithmetic lives in Go** (`internal/render`: render model with every
  coordinate, colour, label and tooltip); the template only emits markup and escapes text with the builtin `html`.
- Visual design follows the old renderer: monospace labels, one row per day (24 px per hour), a ruler with labels every
  6 h, faint guides, the transition table (POSIX + AWS cron), a legend with the base tier marked, tooltips (tier, range,
  starting cron). Days are drawn as merged bucket segments (see preview downsampling); minor ruler ticks appear at
  the bucket size when it is 15–30 minutes. An approximate preview shows a note in the legend line; the transition
  table always stays exact.
- Colours: plain per-tier `fill` rules, repeated in `@media (prefers-color-scheme: dark)`. **No CSS custom
  properties**: librsvg (and other converters / PDF pipelines) render them black.
- **Templates must be gomplate-compatible:** builtins only, and Go executes them with the default strict
  `missingkey=error` like gomplate. The SVG model always emits every key; the report template reads optional keys with
  `index`. Verified byte-identical with gomplate 5.2.0 (`gomplate -c .=model.json -f templates/schedule-svg.tmpl`).
- `docs/office-hours.svg` is the README image, rendered from the issue template; `TestDocsPreview` fails when it is
  stale.

## ASCII preview (implemented 2026-10-24)

- `--output json|ascii`: what stdout carries (default `json`). `--ascii-output-file <path>`: saves the preview
  (atomic, only without errors, like the SVG). With `--output ascii` and errors, stdout stays empty.
- Layout: title (`name - hint`), a ruler (labels every 6 h; `+` every 6 h, `|` every hour, `:` every half hour), one
  row per day (`MON` + one symbol per bucket), a legend with the base tier marked, and the `~` approximation note.
  Rendered through `templates/schedule-ascii.tmpl` on a model computed in `internal/render` (same gomplate rules).
- **Pure ASCII by default (decided 2026-10-24):** block characters (`▒▓█`, lower blocks, `║`, `—`, `≈`) have East Asian
  Width "ambiguous" and turn double-width in CJK terminals, so the ruler and text are always ASCII and the symbols
  come from a preset in `representation.ascii.symbols`:
  - `ascii` (default): `. : = #` by tier order, round robin.
  - `braille`: `⠀ ⣀ ⣤ ⣶ ⣿`, neutral width, so single-width everywhere too. Needs a font with braille glyphs.
  - `styles`: each style's own `symbol` (the built-in ones are ASCII as well). Repeated style symbols only warn, and
    only with this preset.
  - Blocks were rejected as a preset. `--ascii-symbols <preset>` is shorthand for the override; it is applied after
    `--set-config-at` pairs, so the dedicated flag wins.
- Colours: `--color auto|always|never` (default `auto`: only on a terminal, and only when `NO_COLOR` is unset or
  empty). The cell **background** gets the dark theme's tier colour (24-bit, alpha-blended over the dark background);
  the symbol is drawn in the background colour and stays as the fallback without colour. Files get colours only with
  `--color always`.

## Open (fixtures)

- [ ] **Confirm choices made while implementing `expr`** (no fixtures yet; unit tests in `internal/expr`):
      - `to` is lowercase only and needs whitespace on both sides (`TO` and `` `06:00`to`22:00` `` are `invalid-range`).
      - At least one space is required between the end time and the weekdays (`` `22:00`MON `` is `invalid-range`).
      - A range item that spans two lines is `invalid-range`.
      - Repeated days (`MON,MON`, `MON-WED,TUE`) are allowed; painting them twice changes nothing.

## Open ends in the current code

The old code is churn, so don't fix it. The bugs below are useful as negative test cases for the Go implementation.
The rest is cleanup for when the Go tool replaces the old code.

- [ ] **Bugs found by probing `md:extract` (2026-10-24), to use as negative tests:**
      - Alert names handled in two ways by `markdown-tool` (upper-cased native alert vs LinkReference, depending on a
      hyphen).
      - Prose before the code span silently dropped (`> switch at `08:00 MON`` accepted).
      - A standard alert with a code span treated as a tier (`[!NOTE]` → tier `Note`).
      - Unknown BASE values and tier names accepted.
      - Bold markers and alerts nested in lists dropped silently.
      - Frontmatter parsed as a ThematicBreak plus a Setext heading.
- [ ] **`PLAN.md` vs `plan.md`** collide on case-insensitive filesystems. `PLAN.md` (ordered rules) is close to the
      painting model. `plan.md` is the transition spec. Merge them or remove one (ask the user first).
- [ ] **Scratch files are tracked:** `temp.json`, `test.cue`, `test.cue.json`, `original.yaml`, `preview.md`. Ask
      before deleting them.
- [ ] **Obsolete leftovers** from steps 1–3, including `jsonschema = "latest"` (unpinned) in `mise.toml`.
- [ ] **Still open from plan.md §14:** time zone. (CI feedback, milestone 6, is replaced by the error report flags;
      the CLI itself never posts to repos.)

## Future ideas (not scheduled — don't build unasked)

- **GitHub Pages single-page WASM editor** for live editing, preview and validation, giving tighter guidance than the
  issue template. It would output Markdown ready to paste into an issue. The Go/WASM build serves it. CLI, CI and the
  browser must reach identical verdicts, so use shared golden tests.
- **Sphinx docs with custom tier admonitions plus build artifacts.** Schedules render as styled admonitions, and the
  same build emits the canned schedules as artifacts.
  - MyST's GFM `alert` extension handles only NOTE, TIP, IMPORTANT, WARNING and CAUTION. Custom kinds need
    `:::{admonition} …` with `:class:`.
  - Generate the MyST from the canonical JSON with a template, rather than having Sphinx parse the issue Markdown.
    That keeps one parser.
  - Styles come from the config (`representation.styles…`), not duplicated in CSS.
  - Wrap weekday names in MyST's `{abbr}` role (``{abbr}`MON (Monday)` ``).
  - Show the equivalent cron expression as a tooltip.
  - Open: where the docs live, in this repo or in a consumer repo.

## Working conventions

- **Go boilerplate (`40d6d86`):** module `github.com/iilei/canned-schedules`, Go 1.25. `mise.toml` now pins only
  `golangci-lint` and `go`. Tasks: `fmt`, `fmt:check`, `vet`, `lint`, `test` (with `-race`), `coverage`, `build`,
  `tidy`, and `check` (runs all non-mutating checks). `.golangci.yml` is strict (`mnd`, `nonamedreturns`, `godot`,
  `testpackage` outside tests, …). `.goreleaser.yml` handles releases.
  - **All old mise tasks and tools (cue, jq, yq, gomplate, markdown-tool, rumdl, jsonschema) are gone**, so the old
    pipeline no longer runs through mise. The `lint-schedule`/rumdl proposal needs rumdl re-added if it's still wanted.
- **Boilerplate is adapted:** main package `./cmd/canned-schedules`, anchored `.gitignore` binary rule
  (`/canned-schedules`), golangci import prefix `github.com/iilei/canned-schedules`, release texts renamed. Test files
  have relaxed lint rules (`mnd`, `perfsprint`, complexity linters, `fieldalignment`, `rangeValCopy`/`hugeParam`).
- **Go layout** (skeleton since 2026-10-24):
  - `cmd/canned-schedules`: the CLI entry point (`--version`): input reading, report output and exit codes (see
    "Error reporting").
  - `internal/report`: the error report (txt, JSON, built-in Markdown template).
  - `internal/render`: preview downsampling, the SVG and ASCII render models and rendering.
  - `internal/files`: display paths (home as `~`), path-free OS errors, atomic writes; shared by report and SVG.
  - `templates/`: the repo-baked templates (`markdown-gfm.tmpl`, `schedule-svg.tmpl`, `schedule-ascii.tmpl`),
    embedded.
  - `schema/`: a Go package that embeds `config.json`, `config.schema.json`, `meta.schema.json` and
    `output.schema.json` (`go:embed`), plus `Compile`. Its test validates every golden `want.json` against the output
    schema.
  - `internal/diag`: diagnostics, error codes, `Golden()` (fixture format) and `Format()` (for humans).
  - `internal/config`: implemented. Embedded config, `--set-config-at`/`--set-config-value` overrides (replace only,
    index must exist, unknown key is an error), JSON Schema validation (`santhosh-tekuri/jsonschema/v6`), Go-only
    checks (duplicate tier ids/symbols are errors, fewer styles than tiers is a warning).
  - `internal/cli`: flag parsing with the stdlib `flag` package (no CLI dependency, to keep WASM small) and `Run`.
  - `internal/document`: implemented. goldmark-based parser producing the syntax (name, META/BASE/TIER directives,
    list items as code/text tokens) with positions. Handles name/hint rules, marker errors, misplaced directives
    (code spans are masked, so docs that show the syntax in backticks are fine), duplicate blocks, empty tiers,
    missing titles, and non-list content (`invalid-range` in TIER, `meta-schema` in META, `unexpected-content` in BASE).
  - `internal/expr`: implemented. Parses one TIER item into a `Range` (start/end minutes, weekday list with wrap-around
    expanded). Item shape must be code, ` to `, code, whitespace plus weekdays (`invalid-range` otherwise). Times
    `H:MM`/`HH:MM` (`invalid-time`), multiples of `--sample-interval` (`sample-interval`), weekdays per the decided rules
    (`invalid-weekday` at the offending word, `-` or `,`).
  - `internal/schedule`: META validation against the embedded schema (schema errors mapped to the key or value
    position), tier and base resolution, range parsing via `expr`, and painting (`paint.go`: every minute of the week
    starts as base, ranges are painted in source order, then split into per-day runs), transitions with cron
    (`transitions.go`), digests and the self-check against the output schema (`output.go`; a failure is reported as
    `output-schema`, an internal error). `Evaluate` returns the canonical output or diagnostics.
  - `internal/cli/golden_test.go` runs every fixture (101 at the time of the ASCII preview), including `want.preview`
    (the bucketed day rows of the ASCII preview). There is no skip logic any more; a failing fixture fails the build.
- **Issue templates:** `.github/ISSUE_TEMPLATE/canned_schedule_request.md` (with GitHub frontmatter) and
  `.gitlab/issue_templates/canned_schedule_request.md` share one body. `internal/cli/templates_test.go` requires the
  bodies to be identical and the template to evaluate without diagnostics, so keep the template a valid example.
  `.rumdl.toml` ignores MD028/MD032 (and MD025 on GitHub) for them, and excludes golden inputs from linting.
- **JSON key order is part of the output:** `Output` and `Transition` fields are in the documented key order (the
  `Transition` order needs a `fieldalignment` exemption), and the golden runner compares the exact text.
- **Golden fixtures** live in `testdata/golden/` (format in its README), generated by `testdata/golden/gen.py`: `valid/`, `warn/` and `error/` cases, each with
  `input.md`, optional `args` (one arg per line), and `want.json` and/or `want.diag` (`<severity> <line>:<col> <code>`).
  They are the spec. Don't regenerate them from the implementation.
- Generated files (`output/**`, `coverage`, `*coverage.txt`, the binary) are gitignored. Never commit them.
- **Examples in messages and docs:** times sit on a 15-minute grid (`6:15`, `21:45`). Never use `23:59`: it reads
  like a recommended end of day, but the end of a range is exclusive and "until midnight" is written `00:00`. The
  `24:00` error says exactly that.
- Commits follow conventional style (`feat:`, `feat!:` for breaking changes, `docs:`, `chore:`).
- An earlier AI-assisted authoring skill was removed on purpose (`8f556d4`). Don't reintroduce it unasked.
