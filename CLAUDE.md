# Agent steering — canned-schedules (work in progress)

Read this before changing anything. The decisions and the syntax below are the spec; the golden fixtures in
`testdata/golden` are its executable form. Update the "Decisions" and "Open" sections when an item is resolved or a
new one appears.

## Goal

Contributors propose weekly capacity schedules through a GitHub/GitLab **issue template** written in simple Markdown.
The Markdown is parsed, validated and rendered as a preview (ASCII/SVG). The core model has to work with any cloud
provider. The first target is AWS ASG scheduled actions. **Readability for humans comes first**: the input contract
should read like prose, with backticks marking only the values a machine has to read precisely.

## How the design evolved (newest last)

Earlier designs were removed on 2026-10-24 after the Go tool replaced them; they're in the git history if needed.

1. **CUE catalog:** schedules as 48-char slot strings per weekday, rendered by shell scripts and gomplate templates.
2. **Range-based Markdown (`7d9edcb`):** one `## Tier` heading per tier, `**MON-FRI**`, then a list of ranges.
3. **Admonition transitions (`3ec7494`):** `> [!Tier-X]` blocks with transition times, parsed by markdown-tool, jq
   and a regex. Its bugs (case-dependent alert names, silently dropped prose and nested or bold markers, NOTE alerts
   read as tiers, unchecked tier names) are covered by negative golden fixtures.
4. **Current design:** a single Go binary, the namespaced `[!SCHEDULE:*]` syntax with ranges, a painting evaluator, a
   JSON config validated by JSON Schema, error reports and previews. Everything below.

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
  - **Weekday bounds are inclusive** (`MON-FRI` includes FRI), unlike times, whose end is exclusive. Documented in the
    README and the issue templates.
  - **Each abbreviation is written at most once per item (decided 2026-10-24):** `MON,MON`, `WED-WED` and
    `MON-WED,WED` are errors (`duplicate-weekday`, at the second occurrence). One-day ranges are therefore not
    allowed; write the single day.
  - **A day already covered by an earlier part warns** (`repeated-weekday`, at the part that introduces it, once per
    item): `MON-WED,TUE`, `FRI-MON,SUN`. The range still applies.
  - Whitespace from the first weekday name to the end of the line is ignored (`SAT, SUN` and `MON - FRI` are fine).
- **Cron forms are not accepted** as input.
- **`--sample-interval <minutes>`** (renamed from `--granularity`, 2026-10-24, for one vocabulary with
  `--preview-downsample`): default `1`. It must divide 60 evenly (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60). Every
  time must be a multiple of it, otherwise it's an error (`sample-interval`). This is an **input rule**: there is no
  rounding, ever.
- **Item details (decided 2026-10-24):** `to` is lowercase only and needs whitespace on both sides (`TO` and
  `` `06:00`to`22:00` `` are `invalid-range`); at least one space is required between the end time and the weekdays
  (`` `22:00`MON `` is `invalid-range`); a range item that spans two lines is `invalid-range`.
- **No time zones (decided 2026-10-24):** schedules are wall-clock times without a zone, and cron strings carry none.
  Time zones stay out of this tool. UTC is the recommended convention; a team that needs another zone writes a
  separate schedule for it, and downstream tooling (ASG `TimeZone`, EventBridge `ScheduleExpressionTimezone`) sets
  the zone.

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
  - Colours: `#RRGGBB` or `#RRGGBBAA` (`"color": {"light": "#54C6A5D9", "dark": "#9D82EAD9"}`).
  - `symbol` is one character, used by the ASCII preview with the `styles` preset.
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
- **Examples are a separate field (decided 2026-10-24):** `diag.Diagnostic.Example` holds the exact text to type
  (Markdown, backticks included); `Message` is plain prose without "e.g." tails. txt appends `; e.g. <example>`, the
  JSON report and the canonical JSON warnings carry `example`, the Markdown report puts it in its own `~~~~~~markdown`
  fence, and the page shows it as code with a copy button. No control characters or markup inside messages.
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

## Preflight page (implemented 2026-10-24)

- A static page (`web/`) runs the pipeline compiled to WebAssembly (`cmd/canned-schedules-wasm`, a thin
  `syscall/js` wrapper around `internal/web.Evaluate`). Nothing leaves the browser.
- Shows: the SVG week (showcase, rendered with `--svg-theme` matching the page theme), the Markdown as a Sphinx page
  with custom admonitions (the default, left in the switch) or as a GitHub issue (`internal/htmlpreview`), diagnostics that jump to `line:col` in the
  editor, the JSON and the ASCII preview. Options: preview buckets, sample interval, ASCII symbols. Theme toggle
  (system/light/dark, saved in localStorage); `?theme=`, `?view=`, `?tab=`, `?symbols=` and `?preset=` make views
  linkable, and
  `#doc=<base64url>` opens a document (a fragment, so it never reaches the server).
- **Editor (decided 2026-10-24): CodeMirror 6**, in `editor/` (`src/editor.js`, `build.mjs`, `package.json` with
  exact versions and a lockfile; `.npmrc` turns install scripts off). `web:build` runs `npm ci --prefix editor` and
  `node editor/build.mjs dist/site`, which bundles with esbuild into `editor.js` (IIFE, sets
  `globalThis.CannedEditor`) and writes `editor-licenses.txt` for every package with code in the bundle. Only the
  page grows (412 KB minified, ~134 KB gzip); the CLI and `app.wasm` are unaffected.
  - Markdown via `markdownLanguage` plus the list/quote continuation commands, **not `markdown()`**: its defaults pull
    in the HTML, CSS and JavaScript modes (569 KB). Schedule and alert markers get their own colour (they parse as
    links). Colours are `--hl-*` CSS variables, so the theme toggle needs no reconfiguration.
  - Diagnostics: `@codemirror/lint` squiggles (the word at `line:col`, columns in code points) with gutter icons and
    tooltips, plus whole-line backgrounds (`cm-diag-line-error` wins over `-warn`). The JSON tab is a read-only view
    with folding.
  - `app.js` talks to an adapter (`value`, `onChange`, `focus`, `selectAll`, `jumpTo`, `setDiagnostics`); without
    `editor.js` it wraps the plain textarea and `<pre>`, which stay in `index.html` as fallbacks.
  - Versions follow the two-week rule: installed with `npm install --before=<date two weeks back>`.
- Presets: `web/presets/index.json` lists them (id, label), `web/presets/<id>.md` holds the text. `office-hours` is
  the GitLab issue template, copied in by `web:build`, so it never drifts. `TestPresets` (internal/web) requires every
  preset to evaluate without diagnostics and the index and files to match. The dropdown replaces the editor content
  (asking first when there are unsaved edits); Reset is disabled until the editor differs from the selected preset.
- `--svg-theme auto|light|dark` (CLI too): an inline SVG's `prefers-color-scheme` follows the OS, not the page, so
  pages with their own toggle render a fixed palette. SVG rules are scoped under `.canned-schedule` because inline
  SVG styles are global in HTML.
- `internal/htmlpreview`: goldmark + GFM in safe mode (raw HTML dropped, dangerous links removed).
  - GitHub mode: NOTE/TIP/IMPORTANT/WARNING/CAUTION become GitHub callouts (only when the marker stands alone);
    schedule directives stay quotes with the literal marker, as GitHub really shows them.
  - Sphinx mode: all alerts become admonitions; schedule directives get `admonition schedule schedule-tier tier-N`
    with the tier's display name, coloured by `sphinxkit.CSS(cfg, sphinxkit.Page)` (see "Sphinx kit"); weekday names
    inside schedule admonitions get `<abbr>` tooltips. BASE gets the body `myst.BaseText`; a META block with only a
    hint is a paragraph, like in the MyST output.
  - `web/style.css` imitates Furo's admonitions with Furo's variable names, so the generated schedule stylesheet works
    unchanged on the page and in a real Furo build.
- Fonts: Ubuntu + Ubuntu Mono, **self-hosted** in `assets/fonts` (embedded by `assets/embed.go` for the Sphinx kit,
  copied to `dist/site/fonts` by `web:build`; Ubuntu Font Licence, `UFL.txt`): no requests to Google
  (a German court treated Google Fonts loading as a privacy violation), works offline.
  - Ubuntu Mono has **no braille glyphs** and narrow cells (0.5 em); fallback fonts draw braille wider, so braille rows
    overshot the ruler. The ASCII tab therefore always uses `Schedule Braille Mono` (for every preset): a renamed subset (ASCII,
    Latin-1, Braille Patterns, no OpenType features, 16 KB) of Cascadia Mono 2407.24, where every glyph has one width.
    Renamed because the SIL OFL reserves the name "Cascadia Code" for unmodified fonts
    (`fonts/OFL-schedule-braille-mono.txt`). DejaVu Sans Mono was checked first and has no braille.
  - In terminals the user's font decides; terminals keep a fixed cell width, but the font needs braille glyphs.
- Build: `mise run web:build` → `dist/site` (static files, `app.wasm` ~10 MB raw / ~2.4 MB gzip, Go's matching
  `wasm_exec.js`, the GitLab template as `template.md`, `version.txt`). `mise run web:serve [--port N]` serves it on 127.0.0.1 only (default port 8080).
- Parity: `mise run test:wasm` runs all test suites compiled to WebAssembly under Node (pinned in `mise.toml`), with a
  minimal environment because Go's WebAssembly runner caps command line plus environment at a few KB. CI runs it.
- Deploy: `.github/workflows/pages.yml` publishes `dist/site` on pushes to `master` (SHA-pinned
  `upload-pages-artifact`/`deploy-pages`). **One-time setup:** Settings → Pages → Source: "GitHub Actions".
- To verify after the first deploy: whether GitHub Pages serves `app.wasm` compressed (otherwise 10 MB per visit).

## Sphinx kit (implemented 2026-10-24)

- Goal: Sphinx docs (Furo first) get exactly the page's Sphinx look. Decided: CLI flag plus page button, a local
  extension folder rather than a PyPI package, fonts only inside schedule admonitions.
- `templates/sphinx-schedule.css.tmpl` is the **single source** of the schedule admonition styles, rendered by
  `internal/sphinxkit.CSS(cfg, target)` for two targets that differ only in selectors: `Page` (scope
  `.rendered.sphinx`, `:root[data-theme=dark|system]`) and `Furo` (no scope, `body[data-theme="dark"]`, auto =
  `body:not([data-theme="light"])` inside a dark media query). It only sets Furo's variables
  (`--color-admonition-title`, `-title-background`, `--color-admonition-background`, `--icon-admonition-default` for
  the tier swatch) from `--tier`; checked against furo 2025.12.19.
- `templates.Execute` is the shared JSON-shaped template runner (render, report, CSS, kit templates).
- `internal/myst`: a valid request → MyST (H1, META as `Schedule: hint` admonition or a hint paragraph, BASE with
  `BaseText`, TIER items with `{abbr}` weekdays). docutils requires admonition content, hence BaseText. Text goes
  through `diag.Sanitize`, so a newline in a display name can't inject directive options. Needs `colon_fence`.
- `internal/sphinxkit.Kit`: deterministic zip (sorted, fixed timestamps) with README, `_ext/canned_schedules`
  (`__init__.py` loads myst_parser, appends its static dir to `html_static_path`, adds `colon_fence`, adds the CSS),
  the Furo stylesheet, four Ubuntu fonts + UFL, `examples/<slug>.md`, VERSION (tool + config digest).
- Channels: `--myst-output-file`, `--sphinx-kit-output-file` (written in `writeOutputs` like the previews, only
  without errors); page: `cannedSchedules.sphinxKit(markdown, args)` returns a `Uint8Array` or an error string, built
  on click.
- Verified end to end with sphinx 9.1.0, furo 2025.12.19, myst-parser 5.1.0: `sphinx-build -W` clean, screenshots in
  light and dark match the page.

## Open

- Nothing open at the moment.

## Future ideas (not scheduled — don't build unasked)

- **Sphinx build artifacts** (the styling part is done, see "Sphinx kit"): the same docs build emits the canned
  schedules as artifacts; a cron expression as a tooltip; other themes than Furo. Open: where the docs live, in this
  repo or in a consumer repo.
- **Admonition-style SVG** (asked 2026-10-24, deferred): a second SVG showing the request as the page's Sphinx view
  does: a META card ("Schedule: hint"), the BASE card (`myst.BaseText`), one tier-coloured card per TIER with one row
  per range, weekday `<title>` tooltips. Same pattern as the week SVG: a model with all coordinates in
  `internal/render`, a gomplate-compatible `templates/schedule-cards-svg.tmpl`, plain per-tier fills with
  `prefers-color-scheme` and `--svg-theme` (no CSS variables), title and body tints pre-blended with `blend`. Content
  walk as in `internal/myst.Render`. No text wrapping needed (all lines are short and validated); monospace items,
  fixed card width; system fonts (embedding Ubuntu ≈ +60 KB). Channels when built: a CLI flag (only without errors)
  and a page download button. Estimated at about half a day.
- **Schedule catalog and distribution** (from the removed `UTILIZATION.md`): the repo (or a consumer repo) as the
  catalog of approved schedules, e.g. `schedules/<name>/source.md` plus generated `schedule.json`, SVG and ASCII
  previews. CI validates and renders every schedule and publishes the bundle as a versioned artifact (release assets
  or an internal registry such as Artifactory); a docs handbook (see the Sphinx idea) embeds the presets list,
  summaries and previews, linking to the versioned artifacts.

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
  - `internal/htmlpreview`, `internal/web`, `cmd/canned-schedules-wasm`, `web/`: the browser preflight page (see
    "Preflight page").
  - `internal/myst`, `internal/sphinxkit`, `assets/`: MyST output and the Sphinx kit (see "Sphinx kit").
  - `templates/`: the repo-baked templates (`markdown-gfm.tmpl`, `schedule-svg.tmpl`, `schedule-ascii.tmpl`,
    `sphinx-schedule.css.tmpl`), embedded.
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
- Generated files (`coverage`, `*coverage.txt`, the binary) are gitignored. Never commit them.
- **Examples in messages and docs:** times sit on a 15-minute grid (`6:15`, `21:45`). Never use `23:59`: it reads
  like a recommended end of day, but the end of a range is exclusive and "until midnight" is written `00:00`. The
  `24:00` error says exactly that.
- Commits follow conventional style (`feat:`, `feat!:` for breaking changes, `docs:`, `chore:`).
- An earlier AI-assisted authoring skill was removed on purpose (`8f556d4`). Don't reintroduce it unasked.
