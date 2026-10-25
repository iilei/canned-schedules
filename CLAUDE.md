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
- **Digests (decided, implemented).** The output carries `digests.schedule`: SHA-256 of compact `{"base", "week"}`
  JSON, written `sha256:<hex>`. Name, summary, annotations, prose and range wording that paints the same minutes don't
  change it. `base` is part of it because consumers use it as the fallback, for example when `transitions` is `[]`.
  - **No request digest (dropped 2026-10-25):** a hash of the input bytes earned little. Provenance belongs in git
    SHAs or in an `x-*` META key (an issue link, a commit), and a bot that must pin an issue revision hashes
    the body itself. The `digests` object stays, so a digest can be added later without reshaping the output.
  - Still proposed: storing the raw input next to the output (`schedules/<name>/source.md` + `schedule.json`).
    Consumers read only the output; the raw Markdown is never part of the contract.
- **Linting (proposed).** rumdl has no custom rules, and a markdownlint-cli2 JS rule would be a second parser.
  - Use rumdl for generic style only. Add a per-file ignore for MD028 on schedule files, because adjacent alerts
    trigger it.
  - Add a `lint-schedule` mise task that wraps the Go validator and prints rumdl's concise format,
    `file:line:col CODE message`.

## Input syntax

No frontmatter. Everything uses one grammar: an alert whose type is the `SCHEDULE` namespace, a hyphen and a kind.

```markdown
# Schedule-Name

> [!SCHEDULE-META]
> - schema-version: `1`
> - x-owner: `team-xyz`

> [!SCHEDULE-BASE] Minimum

> [!SCHEDULE-STATE] Regular
> - `06:00` to `22:00` MON-FRI
> - `10:00` to `00:00` SAT,SUN

> [!SCHEDULE-STATE] Maximum
> - `22:00` to `02:00` FRI
```

### Structure (decided)

- **Namespace: a fixed allowlist, not configurable** (decided 2026-10-24). **Only `SCHEDULE` for now** (decided
  2026-10-25: keep the spec tight; the aliases `CANNED-SCHEDULE`, `TO-SCHEDULE`, `CAN-SCHED` and `CONFIG-SCHED` were
  dropped). `document.Namespaces` stays a list, so an alias can come back. It is read case-insensitively. Adding an
  alias takes a merge request and a new release; aliases would be normalised to `SCHEDULE`.
  - **Hyphen, not colon (decided 2026-10-25):** `[!SCHEDULE-STATE]`. Callout tools that accept custom types (Obsidian
    convention: rehype-callouts `[\w-]+`, mkdocs-callouts, Obsidian) reject `:`, so the hyphen lets documents render
    as callouts elsewhere. `document.SplitMarker` is the one place that recognises directives (the parser and the
    page's preview use it). The old colon form (`[!SCHEDULE:STATE]`) is an `invalid-marker` error pointing to the
    hyphen.
  - **Other alerts are prose by default** (NOTE, TIP, custom callouts such as `[!custom-question]`), including
    near-misses of the namespace (`[!SCHEDUEL-STATE]`): no fuzzy matching in the parser.
  - **`--error-unless-alert-pattern <regex>` (decided 2026-10-25)** makes this strict: every alert that isn't a
    schedule directive must match, otherwise `disallowed-alert` at the marker. Go RE2 syntax, `(?i)` for case, and the
    tool anchors it (`^(?:…)$`), because an unanchored `^A|B$` silently splits the anchors. Schedule directives are
    always allowed, so forgetting `SCHEDULE-` in the pattern can't reject every directive. An invalid pattern is
    `invalid-flag`. CI and issue bots should set it (e.g. `'(?i)NOTE|TIP|IMPORTANT|WARNING|CAUTION'`) to catch
    namespace typos. A customisable prefix (env var or flag) was considered and rejected: hidden state, and the page
    and templates couldn't follow it.
  - An unknown kind in a known namespace (`SCHEDULE-TEIR`) is a hard error (`unknown-kind`).
- **Title on the marker line** carries the name (`STATE`, `BASE`). GitHub and GitLab show custom alerts as literal
  text; the Obsidian callout convention (custom type plus title) matches this shape, and our goldmark extension
  defines the grammar.
- **META** is a list of `key: `value`` items, in one form only. A list renders the same everywhere, while `>` lines
  without a list form one paragraph, and whether they break depends on the renderer.
  - Allowed keys: `schema-version` and `x-*` (free-form). Nothing else for now. Shortened from `x-custom-*` on
    2026-10-25: `x-` is the OpenAPI convention, and RFC 6648's objection to `X-` headers doesn't apply, because these
    keys are user bookkeeping and never become standard. Keys without `x-` stay reserved for the spec. Old
    `x-custom-*` keys still validate as `x-` keys named `custom-*`.
  - **The META block and its `schema-version` are both optional.** Whenever `schema-version` is missing, whether the
    block is absent or just lacks the key, the version built into the binary (currently `"1"`) applies.
  - **Text after the META marker is the schedule's summary** (`[!SCHEDULE-META] Office hours only`). It is trimmed, then
    validated with the same rule as the name (see below), and output as `summary`. A META block with a summary and no items is
    allowed.
  - **`summary`, not `hint` (renamed 2026-10-25):** "hint" read like advice for the reader; the field is a one-line
    subtitle of at most 80 characters. `summary` follows the OpenAPI split (short `summary`, long `description`) and
    keeps `description` free for long text later. Renamed everywhere: the output key, `invalid-summary`, Go fields,
    render models and docs.
  - `x-*` keys are allowed **only in META**. Their values are uninterpreted strings, passed through to the output
    (for example under `annotations`). They must never affect the domain model.
- **STATE items as ranges:** in each item, the times are code spans, `to` is a literal keyword that is part of the
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
- **No-break space is whitespace (decided 2026-10-25):** `document.IsSpace` (space, tab, U+00A0) is the one rule for
  every place the grammar allows whitespace: trimming of name, summary and titles (with matching columns), around ` to `,
  before and inside weekday expressions, around the colon of META keys. Copy and paste brings U+00A0 in, and it
  renders like a space. Other invisible characters (zero-width space, …) are not whitespace and stay errors, escaped in
  messages. U+202F (narrow no-break space) only on request. Fixtures: `valid/nbsp-as-whitespace`,
  `error/zero-width-space-in-weekdays`.
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
  with later ranges winning where they overlap. Every minute gets exactly one state. The transitions for ASG scheduled
  actions are derived from the painted week as an output, not written by the user.
- **BASE via CLI flags:**
  - `--require-base true`: the document must contain `[!SCHEDULE-BASE]`. `--base` must not be given (error).
  - `--require-base false` (**the default**): the base comes from the first of these that exists: the document's
    BASE, then `--base <state>` (must name a valid state), then the first state (index 0) in the config.
- **Preview downsampling (visual only, decided 2026-10-24; supersedes the earlier GCD-based grid):** previews (SVG
  and ASCII) show buckets of N minutes. (none) or `--preview-downsample` → 15 (96 columns, `MON` + 96 = 100
  characters); `--preview-downsample=N` → N (must divide 60; the value must be attached with `=`);
  `--no-preview-downsample` → 1 (1440 columns). Combining the last two is `invalid-flag`.
  - **Never applied to the canonical JSON** (week, transitions, digests stay exact) and never to input reading.
  - A bucket shows the state covering most of its minutes; ties go to the state that comes later in the config. When
    buckets mix states, a `preview-approximate` warning says how many; it appears only when a preview is requested.
  - Anyone who needs exact previews sets `--sample-interval` to a multiple of the bucket size (e.g. 15).
- **Warnings in the canonical JSON (decided 2026-10-24):** `warnings` lists every warning (`code`, `message`, and
  `line`/`column`/`excerpt` when positioned), sorted by position. No file names, so the output doesn't depend on where
  the file lives. Not part of the schedule digest. Golden fixtures don't pin warning messages (`"…"`).

### State names (decided)

- **STATE, not TIER (decided 2026-10-25):** the model maps every minute to one value from an ordered list, which fits
  more than capacity tiers (on-call teams, support hours, maintenance modes). "Tier" implied ranking and clashed with
  escalation tiers in on-call. Renamed everywhere: the kind (`[!SCHEDULE-STATE]`), `state` in `week` and
  `transitions`, the config's `states` and `representation.styles.states`, error codes (`empty-state`,
  `unknown-state`, …), CSS classes (`schedule-state`, `state-N`, `--state`). No `TIER` alias. Config order stays a
  technical detail (default base, preview tie-breaking), not "more" or "less". The defaults remain the capacity
  example (`Off`, `Minimum`, `Medium`, `Maximum`).

- Pattern `^[A-Za-z][A-Za-z0-9]*(?:-[A-Za-z0-9]+)*$`, 1–32 chars, ASCII, hyphen as the only separator. Names work
  directly as MyST/CSS classes.
- Case-insensitive. Canonical key = upper-cased name. Reject configs whose state ids collide after upper-casing. Display
  name = `display-name` from the config.
- Rejected: base64-encoding names (`markdown-tool` upper-cased them, which corrupts base64, and the result is
  unreadable).

## Config (decided)

Draft files (validated with `mise exec -- jsonschema validate`): `schema/config.schema.json`, `schema/config.json`
(defaults taken from the old CUE values), `schema/meta.schema.json`. JSON Schema draft 2020-12.

- **A repo-level JSON file with a JSON Schema next to it**, both embedded in the binary with `go:embed`. The built-in
  config is validated by **tests**, not at runtime.
- **No custom schema.** The schema is fixed. Extension happens only through `x-*` keys in document META.
- **Shape:** domain and representation are kept separate.

  ```text
  schema-version            const "1"
  states[]                   {id, display-name}: ordered, index 0 = default base
  representation.theme      {light, dark} × {background, label}: page-level colours
  representation.ascii      {symbols}: ascii | braille | styles
  representation.styles.states[]
                            {symbol, color: {light, dark}}: round robin, no state id
  ```

  - Styles carry **no state id**, so round robin works: state *i* gets `styles.states[i mod len]`. Users steer the
    mapping by ordering both lists.
  - Colours: `#RRGGBB` or `#RRGGBBAA` (`"color": {"light": "#54C6A5D9", "dark": "#9D82EAD9"}`).
  - `symbol` is one character, used by the ASCII preview with the `styles` preset.
  - Every object has `additionalProperties: false`. `x-*` is not allowed in the config.
- **Checks JSON Schema can't express** (done in Go, covered by tests):
  - state ids unique after upper-casing;
  - repeated style symbols are a **warning** (`config-duplicate-symbol`, decided 2026-10-24: permissive), because
    colours can still tell states apart;
  - **WARN** if `len(styles) < len(states)`: round robin then repeats symbols and colours, so the ASCII grid and legend
    become ambiguous.
- **No namespace in the config** any more (`parser.namespace` was removed). The namespaces are a constant in the Go code
  (`document.Namespaces`). Each one matches `^[A-Z](?:-?[A-Z0-9])*$`: a letter first, a letter or digit last, single hyphens between.
- **State id pattern:** `^[A-Za-z](?:-?[A-Za-z0-9])*$`, max 32 chars. It's the same rule as before, written in the same
  style as the namespace rule.
- **Document META** (`schema/meta.schema.json`):
  - `schema-version` is optional, defaulting to the built-in version. It is the **string** `"1"`, because every META
    value comes from a code span.
  - Otherwise only `x-<name>` keys, with `<name>` lowercase letters/digits and single hyphens, up to 40 chars.
  - At most 40 custom keys (`maxProperties: 41`). Each value is a string of at most 48 chars, so 40 × 48 = 1920 chars,
    which stays under the 2 KB budget for ASCII. JSON Schema counts code points, not bytes, so non-ASCII values can
    exceed 2 KB slightly. That was accepted in exchange for checking everything in the schema.
- **Overrides via repeatable flag pairs** `--set-config-at <path>` / `--set-config-value <json>`. File paths are not
  accepted. Users who want a file can pass `"$(cat file.json)"`.

  ```sh
  --set-config-at 'representation.styles.states[0]' \
  --set-config-value '{"symbol": "░", "color": {"light": "#A7DCC840", "dark": "#B9A5FF40"}}'
  ```

  - The pairs are matched by position: the n-th `-at` goes with the n-th `-value`. Unequal counts are an error.
  - The path syntax is dot-separated keys plus `[index]`.
  - The full config is validated against the embedded schema **after** all overrides are applied, when the tool is
    invoked.
- The `_` protected-key convention is **dropped**.
- **Override semantics:** replace the value at the path, never deep-merge. Pairs are applied in order. An index must
  already exist, and an unknown key in the path is an error, so a typo can't create a key. To add or remove states,
  replace the whole list (`--set-config-at states`).
- **Out of scope:** capacity numbers and any other provider-specific settings (for example ASG `min`/`desired`/`max`).
  **State ids are the integration edge:** downstream tooling attaches provider config to state ids. Never add such fields
  to the config or the output.
- **`schema-version` is a string** (`"1"`) everywhere: config, META and output.
- **Schema `$id`s** point to the raw files on GitHub, for example
  `https://raw.githubusercontent.com/iilei/canned-schedules/master/schema/config.schema.json`. `config.json` references
  it through `$schema`. These are placeholders, possibly moving to GitHub Pages later. A branch URL can change under
  you, so consider tag-pinned URLs once there are releases.

### Transitions in the output (decided 2026-10-24)

- Part of the **canonical output** (`transitions`), derived from the painted week. `week` stays the primary data,
  because transitions alone don't give the state in effect right now.
- Each entry: `time`, `days` (**full list**, Monday first), `state`, and `cron` with two dialects: `posix` (five
  fields, numeric weekdays, 0 = SUN; EC2 Auto Scaling) and `aws` (six-field EventBridge format, weekday names). Real
  Quartz (with a seconds field) only if someone needs it.
- Grouped by (time of day, state); ordered by first minute in the week, then state. Cron day lists are Sunday-first and
  never wrap around the week. All seven days become `*`. No time zone in cron strings.
- Implemented in `internal/schedule/transitions.go`, with an independent reference in `testdata/golden/gen.py`.

### Document structure rules (decided 2026-10-24)

- **The H1 is mandatory** and becomes `name` in the output. There must be exactly one (`missing-name`,
  `duplicate-name`).
- **Name and summary share one text rule** (`invalid-name`, `invalid-summary`): trimmed, 1–80 chars, ASCII; starts with a
  letter; may contain letters, digits, spaces and `-`; ends with a letter or digit. Regex:
  `^[A-Za-z](?:[A-Za-z0-9 -]{0,78}[A-Za-z0-9])?$`.
- **A marker with a namespace but no kind** (`[!SCHEDULE]`, `[!SCHEDULE-]`) is an error (`missing-kind`).
- **At most one META block and one BASE block** (`duplicate-block`, reported at the second one).
- **A STATE block must contain at least one range** (`empty-state`).
- **Directives only count at the top level.** A directive nested in a list or another container, or a bold-wrapped
  marker (`> **[!SCHEDULE-STATE]**`), is an error (`misplaced-directive`). It is never silently ignored.

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
  6 h, faint guides, the transition table (POSIX + AWS cron), a legend with the base state marked, tooltips (state, range,
  starting cron). Days are drawn as merged bucket segments (see preview downsampling); minor ruler ticks appear at
  the bucket size when it is 15–30 minutes. An approximate preview shows a note in the legend line; the transition
  table always stays exact.
- Colours: plain per-state `fill` rules, repeated in `@media (prefers-color-scheme: dark)`. **No CSS custom
  properties**: librsvg (and other converters / PDF pipelines) render them black.
- **Templates must be gomplate-compatible:** builtins only, and Go executes them with the default strict
  `missingkey=error` like gomplate. The SVG model always emits every key; the report template reads optional keys with
  `index`. Verified byte-identical with gomplate 5.2.0 (`gomplate -c .=model.json -f templates/schedule-svg.tmpl`).
- `docs/office-hours.svg` is the README image, rendered from the issue template; `TestDocsPreview` fails when it is
  stale.

## ASCII preview (implemented 2026-10-24)

- `--output json|ascii`: what stdout carries (default `json`). `--ascii-output-file <path>`: saves the preview
  (atomic, only without errors, like the SVG). With `--output ascii` and errors, stdout stays empty.
- Layout: title (`name - summary`), a ruler (labels every 6 h; `+` every 6 h, `|` every hour, `:` every half hour), one
  row per day (`MON` + one symbol per bucket), a legend with the base state marked, and the `~` approximation note.
  Rendered through `templates/schedule-ascii.tmpl` on a model computed in `internal/render` (same gomplate rules).
- **Pure ASCII by default (decided 2026-10-24):** block characters (`▒▓█`, lower blocks, `║`, `—`, `≈`) have East Asian
  Width "ambiguous" and turn double-width in CJK terminals, so the ruler and text are always ASCII and the symbols
  come from a preset in `representation.ascii.symbols`:
  - `ascii` (default): `. : = #` by state order, round robin.
  - `braille`: `⠀ ⣀ ⣤ ⣶ ⣿`, neutral width, so single-width everywhere too. Needs a font with braille glyphs.
  - `styles`: each style's own `symbol` (the built-in ones are ASCII as well). Repeated style symbols only warn, and
    only with this preset.
  - Blocks were rejected as a preset. `--ascii-symbols <preset>` is shorthand for the override; it is applied after
    `--set-config-at` pairs, so the dedicated flag wins.
- Colours: `--color auto|always|never` (default `auto`: only on a terminal, and only when `NO_COLOR` is unset or
  empty). The cell **background** gets the dark theme's state colour (24-bit, alpha-blended over the dark background);
  the symbol is drawn in the background colour and stays as the fallback without colour. Files get colours only with
  `--color always`.

## Preflight page (implemented 2026-10-24)

- A static page (`web/`) runs the pipeline compiled to WebAssembly (`cmd/canned-schedules-wasm`, a thin
  `syscall/js` wrapper around `internal/web.Evaluate`). Nothing leaves the browser.
- Shows: the SVG week (showcase, rendered with `--svg-theme` matching the page theme), the Markdown as a Sphinx page
  with custom admonitions (the default, left in the switch) or as a GitHub issue (`internal/htmlpreview`), diagnostics that jump to `line:col` in the
  editor, the JSON and the ASCII preview. Options: preview buckets, sample interval, ASCII symbols. Theme toggle
  (system/light/dark, saved in localStorage); `?preset=`, `?theme=`, `?view=`, `?tab=`, `?downsample=`,
  `?sample-interval=` and `?symbols=` make views linkable, and `#doc=<base64url>` opens a document (a fragment, so it
  never reaches the server). **Copy link** builds that URL from the current state (`permalink()` in `app.js`: every
  control explicitly, the theme only when not "system", `#doc=` only when the editor differs from the preset), copies
  it and puts it in the address bar.
- **Loading overlay:** `#hud` covers the page until the WebAssembly checker runs (`start()` hides it), and stays with
  the reason when loading fails (no WebAssembly, `app.wasm` not loadable). `fetchWasm()` streams the download to show
  the megabytes loaded, with a total and percentage only when the response isn't content-encoded (Content-Length
  would count compressed bytes), then compiles from the bytes. The note says it keeps working offline in this tab;
  no service worker, so a reload offline isn't promised. The logo links to the page itself
  (`location.pathname`, no query or fragment) to start over.
- **Resizable editor:** a splitter between the panes (`role="separator"`) sets the editor's share of the width
  (`--editor-width`, 20–70 %, default 33.3 %): drag, arrow keys or Home/End, double-click to reset. Saved in
  localStorage as a per-browser convenience, not part of the permalink. Hidden below 1100 px, where the panes stack.
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
  preset to evaluate without diagnostics and the index and files to match. **Easter egg:** `yolo` is marked
  `"hidden": true`, so it stays out of the dropdown and only opens by link (`?preset=yolo`): the week spells YOLO in
  a 3×5 pixel font. Keep it; it is not dead code. The dropdown replaces the editor content
  (asking first when there are unsaved edits); Reset is disabled until the editor differs from the selected preset.
- `--svg-theme auto|light|dark` (CLI too): an inline SVG's `prefers-color-scheme` follows the OS, not the page, so
  pages with their own toggle render a fixed palette. SVG rules are scoped under `.canned-schedule` because inline
  SVG styles are global in HTML.
- `internal/htmlpreview`: goldmark + GFM in safe mode (raw HTML dropped, dangerous links removed).
  - GitHub mode: NOTE/TIP/IMPORTANT/WARNING/CAUTION become GitHub callouts (only when the marker stands alone);
    schedule directives stay quotes with the literal marker, as GitHub really shows them.
  - Sphinx mode: all alerts become admonitions; schedule directives get `admonition schedule schedule-state state-N`
    with the state's display name, coloured by `sphinxkit.CSS(cfg, sphinxkit.Page)` (see "Sphinx kit"). BASE gets
    the body `myst.BaseText`; a META block without items keeps an invisible `&nbsp;` body, like in the MyST output.
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
  - **Reproducible:** `mise run fonts:braille` runs `assets/build-schedule-braille-mono.py`. It downloads the
    Cascadia release, checks the zip's SHA-256, runs pyftsubset with the original arguments, renames the font and
    sets a fixed `head.modified`. With the pinned fontTools 4.55.3 and brotli 1.2.0 the output is byte-identical to
    the committed font. The script sits outside `assets/fonts`, so it is neither embedded nor published.
  - In terminals the user's font decides; terminals keep a fixed cell width, but the font needs braille glyphs.
- Build: `mise run web:build` → `dist/site` (static files; `web/style.css` stays readable in the repo and is minified
  into the build by the editor's pinned esbuild; `app.wasm` ~10 MB raw / ~2.4 MB gzip, Go's matching
  `wasm_exec.js`, the GitLab template as `template.md`, `version.txt`, which identifies the deployed build but is not shown on the page). `mise run web:serve [--port N]` serves it on 127.0.0.1 only (default port 8080).
- Parity: `mise run test:wasm` runs all test suites compiled to WebAssembly under Node (pinned in `mise.toml`), with a
  minimal environment because Go's WebAssembly runner caps command line plus environment at a few KB. CI runs it.
- Deploy: `.github/workflows/pages.yml` publishes `dist/site` on pushes to `master` (SHA-pinned
  `upload-pages-artifact`/`deploy-pages`). Live at https://iilei.github.io/canned-schedules/ since 2026-10-25 (Pages source "GitHub Actions", HTTPS
  enforced).
- Verified on the first deploy: GitHub Pages serves `app.wasm` gzip-compressed (about 3 MB over the wire for the
  11.5 MB file), so the loading overlay shows "MB loaded" without a total there.

## Sphinx kit (implemented 2026-10-24)

- Goal: Sphinx docs (Furo first) get exactly the page's Sphinx look. Decided: CLI flag plus page button, a local
  extension folder rather than a PyPI package, fonts only inside schedule admonitions.
- `templates/sphinx-schedule.css.tmpl` is the **single source** of the schedule admonition styles, rendered by
  `internal/sphinxkit.CSS(cfg, target)` for two targets that differ only in selectors: `Page` (scope
  `.rendered.sphinx`, `:root[data-theme=dark|system]`) and `Furo` (no scope, `body[data-theme="dark"]`, auto =
  `body:not([data-theme="light"])` inside a dark media query). It only sets Furo's variables
  (`--color-admonition-title`, `-title-background`, `--color-admonition-background`, `--icon-admonition-default` for
  the state swatch) from `--state`; checked against furo 2025.12.19.
- `templates.Execute` is the shared JSON-shaped template runner (render, report, CSS, kit templates).
- `internal/myst`: a valid request → MyST (H1, META always as a `Schedule: summary` admonition, BASE with
  `BaseText`, STATE items written as in the request). docutils requires admonition content, hence BaseText. Text goes
  through `diag.Sanitize`, so a newline in a display name can't inject directive options. Needs `colon_fence`.
  - **META is always an admonition (decided 2026-10-25):** a META block without items gets `myst.EmptyBody`
    (`&nbsp;`) as its body, because docutils rejects an admonition without content, and the class `schedule-empty`.
    The shared stylesheet collapses such a box to its title bar (body hidden, no bottom padding). A class, not a
    `data-*` attribute: Sphinx's admonition takes only `:class:` and `:name:`, and CSS can't select a paragraph that
    holds only `&nbsp;`. Earlier a summary-only META became a
    plain paragraph, which looked inconsistent next to META blocks with items. Authors never type the body: requiring
    a body in the input was rejected (an invisible character, and only docutils needs it).
  - **No weekday tooltips (dropped 2026-10-25):** `{abbr}` roles (and `<abbr>` in the page's Sphinx view) made items
    hard to read, and expanding the fixed English abbreviations told readers almost nothing.
- `internal/sphinxkit.Kit`: deterministic zip (sorted, fixed timestamps) with README, `_ext/canned_schedules`
  (`__init__.py` loads myst_parser, appends its static dir to `html_static_path`, adds `colon_fence`, adds the CSS),
  the Furo stylesheet, all six Ubuntu fonts the page uses + UFL, `examples/<slug>.md`, VERSION (tool + config digest).
- Channels: `--myst-output-file`, `--sphinx-kit-output-file` (written in `writeOutputs` like the previews, only
  without errors); page: **Download MyST** (`myst` in the evaluate result, like the SVG) and
  `cannedSchedules.sphinxKit(markdown, args)`, which returns a `Uint8Array` or an error string and is built on click.
- **MyST is build output (decided 2026-10-25):** users edit and validate the GFM request; docs builds regenerate the
  MyST from it and never edit it by hand. The README's "From request to docs page" recipe shows the CI loop, the
  choice between `.gitignore` and a `git diff --exit-code` drift check, and toctree pages.
- Verified end to end with sphinx 9.1.0, furo 2025.12.19, myst-parser 5.1.0: `sphinx-build -W` clean, screenshots in
  light and dark match the page.

## Licence and releases (decided 2026-10-25)

- **Licence: FSL-1.1-ALv2** (`LICENSE.md`, the fsl.software template, unchanged except the copyright line): any use,
  internal company use included, except a competing commercial product or service; each release becomes Apache-2.0
  two years after publication. Chosen for adoption while keeping room to earn from the concept. The name and logo
  aren't licensed (the licence's Trademarks clause). No CLA for now. Third-party licences ship as
  `THIRD_PARTY_LICENSES.md` in every archive (`mise run licenses`: Go modules in the binary, the Go standard library,
  the embedded fonts).
- **goreleaser** (pinned in `mise.toml`, no goreleaser-action): `.goreleaser.yml` builds linux amd64/arm64/armv7, a
  darwin universal binary and windows amd64/arm64 into `dist/release` (so `--clean` spares `dist/site`), reproducibly
  (`-trimpath`, commit timestamps). It GPG-signs every archive and `checksums.txt` and creates the release as a
  **draft**. `mise run release:snapshot` builds locally without signing or publishing.
- **`.github/workflows/release.yml`** on `v*` tags: build (goreleaser, key from `crazy-max/ghaction-import-gpg`),
  smoke (each native archive verified against `checksums.txt` and run on its runner: ubuntu x86_64 and arm64, macOS,
  windows amd64 and arm64; armv7 has no runner), publish (a `.sha256` sidecar per archive, then the draft is
  published; tags with `-` stay pre-releases, never "latest"). Secrets: `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`.
- **CI matrix:** `check.yml` also builds and tests on ubuntu, macos and windows (Go only). `.gitattributes` forces LF
  so Windows checkouts don't break the golden fixtures; `assets/fonts` stays byte-identical. Tests avoid OS
  specifics: drive letters for absolute paths, no permission bits on Windows, diagnostic codes instead of OS error
  texts.

## Open

- Nothing open at the moment.

## Future ideas (not scheduled — don't build unasked)

- **Sphinx build artifacts** (the styling part is done, see "Sphinx kit"): the same docs build emits the canned
  schedules as artifacts; a cron expression as a tooltip; other themes than Furo. Open: where the docs live, in this
  repo or in a consumer repo.
- **Admonition-style SVG** (asked 2026-10-24, deferred): a second SVG showing the request as the page's Sphinx view
  does: a META card ("Schedule: summary"), the BASE card (`myst.BaseText`), one state-coloured card per STATE with one row
  per range, weekday `<title>` tooltips. Same pattern as the week SVG: a model with all coordinates in
  `internal/render`, a gomplate-compatible `templates/schedule-cards-svg.tmpl`, plain per-state fills with
  `prefers-color-scheme` and `--svg-theme` (no CSS variables), title and body tints pre-blended with `blend`. Content
  walk as in `internal/myst.Render`. No text wrapping needed (all lines are short and validated); monospace items,
  fixed card width; system fonts (embedding Ubuntu ≈ +60 KB). Channels when built: a CLI flag (only without errors)
  and a page download button. Estimated at about half a day.
- **MyST input** (asked 2026-10-25, deferred; keep the input tight until a real need appears): only worth it when
  MyST is the source of truth, e.g. schedules maintained inside a Sphinx handbook. Otherwise GFM stays the only
  input. Issue templates don't render MyST, a second grammar would double the parser, positions and fixtures, and
  `--myst-output-file` already covers docs from the GFM source. If built:
  - **Explicit flag** `--input-format gfm|myst` (default `gfm`), never auto-detected.
  - **A front end only:** a MyST reader that produces the same syntax as `internal/document`, so evaluation, output
    and digests stay shared.
  - **Accepts only the shape `internal/myst` emits** (round trip). Test it by converting every golden input to MyST
    and back.
  - **Digests:** the same schedule in both flavours gets the same schedule digest.
- **First day of the week in previews** (asked 2026-10-25, deferred; worth it once users in regions with Sunday- or
  Saturday-first weeks appear). The setting is `representation.week-start` with `MON` (default, ISO 8601), `SUN` or
  `SAT`, plus a `--preview-week-start <day>` shorthand. The shorthand is applied after `--set-config-at`, like
  `--ascii-symbols`. On the page it is a dropdown plus `?weekstart=`.
  - **Changes only the order of the day rows** in the SVG, ASCII and page previews.
  - **The canonical JSON, the digest, the weekday syntax and `want.json` stay unchanged**, so the same schedule
    never hashes differently depending on who renders it.
  - **The SVG transition table keeps its canonical order**, so it matches the JSON.
  - A range that crosses midnight at the week boundary wraps from the last row to the first; SUN → MON already does
    that today.
  - With `SUN`, the weekend lands in the first and last rows, so `MON` stays the default.
  - About 1–2 h of work, including new `want.preview` fixtures via `gen.py`.
- **MyST generated during the Sphinx build** (asked 2026-10-25, deferred). The kit's extension would run the CLI
  when the build starts (`builder-inited`), for `canned_schedules_sources = "schedules/*/source.md"` in `conf.py`,
  and write the MyST pages before Sphinx reads them. The Go tool still parses and validates, and an invalid request
  fails the build, so nothing drifts. That is the difference from the rejected Python transform that would have
  rendered GFM inside Sphinx without validating it.
  - Cost: the binary must be available where the docs are built.
  - The extension compares the tool version with the kit's `VERSION` and warns when they differ.
  - Until then, the README's "From request to docs page" recipe (a CI loop before `sphinx-build`) covers this.
- **Offline page (service worker)** (asked 2026-10-25, deferred): after loading, the page already works offline in its
  tab, and repeat visits reuse the HTTP cache (Pages: 10 minutes fresh, then a cheap revalidation). Offline use is
  the CLI's job; the footer points to it. If ever built, avoid stale results, because syntax changes break documents:
  network-first for `index.html`, `app.js` and `presets/index.json`; `app.wasm` cached under its build version so
  an old checker never meets new page code; the status line says "offline copy from <date>" when the cache serves.
  About half a day including offline and update testing.
- **Schedule catalog and distribution** (from the removed `UTILIZATION.md`): the repo (or a consumer repo) as the
  catalog of approved schedules, e.g. `schedules/<name>/source.md` plus generated `schedule.json`, SVG and ASCII
  previews. CI validates and renders every schedule and publishes the bundle as a versioned artifact (release assets
  or an internal registry such as Artifactory); a docs handbook (see the Sphinx idea) embeds the presets list,
  summaries and previews, linking to the versioned artifacts.
- **TOML companion as a converter** (asked 2026-10-25, deferred; worth it for a catalog or tool-generated schedules,
  not for the issue template). A `toml-to-md` helper, **not a second input format**: Markdown stays the only contract,
  so the parser, positions, digests and fixtures stay single.
  - **Shape:** top-level `schema-version`, `name`, `summary`, `base`; `[x]` for `x-*` keys; states as an ordered
    `[[state]]` array (`id`, `ranges`), because TOML tables are unordered and painting depends on source order. Each
    range is a **triplet** `["14:00", "22:00", "MON-FRI"]` (start, end, weekday expression): the array separates the
    parts, so the `to`/whitespace rules (`invalid-range`) don't apply, and every part has its own position. Times and
    weekday expressions keep their grammar and error codes (`internal/expr`); weekdays stay one expression string,
    not a list. Inline tables (`{from, to, days}`) rejected: verbose, one line only in TOML 1.0, misspellable keys.
    Times always quoted; reject TOML local times. Unknown keys are errors. YAML rejected: YAML 1.1 reads `06:00` as
    base-60 `360`.
  - **JSON Schema for the TOML** (`schema/toml-request.schema.json`, draft 2020-12): one file for the Go converter
    (decode, then validate with `santhosh-tekuri/jsonschema`) and for editors via Tombi (completion, hover, errors).
    Shape only: `additionalProperties: false`, triplets as `prefixItems` + `minItems: 3` + `items: false`, `x-*`
    rules as in the META schema, plus a simple time pattern for feedback while typing. Weekday expressions,
    `--sample-interval`, state existence and duplicates stay in Go (no second grammar source).
  - **Tombi as a dev tool** (pinned in `mise.toml`, like rumdl for Markdown; never at runtime). Key order through
    `x-tombi-table-keys-order: "schema"` (scalars, `[x]`, `[[state]]`) is cosmetic: `tombi format` only, never a CI
    error, since the converter's output doesn't depend on key order. **Arrays are never sorted** (`[[state]]` and
    `ranges` order is painting order): no `x-tombi-array-values-order`, and the schema's `description` says why.
    Verify extension names, schema association (`#:schema` or `tombi.toml`) and whether `tombi lint` reports key
    order against Tombi's current docs before relying on them.
  - **Exposed as a subcommand** `canned-schedules toml-to-md <file.toml>` (stdout or `--md-output-file`, atomic). The
    converter checks only the TOML shape (schema above), writes canonical Markdown (fixed block order, `HH:MM`, items
    as `` `14:00` to `22:00` MON-FRI ``), runs the normal pipeline on it and maps diagnostics back to TOML positions
    via a table from Markdown item parts to TOML array elements.
  - Optional `md-to-toml` from the `document` syntax (prose and comments are lost; document that). Test: every golden
    input MD → TOML → MD gives the same `want.json`.
  - Pick the Go TOML library by its position info: line numbers for keys and for **array elements**; without element
    positions, errors fall back to the line of the `ranges` key. About half a day.

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
    checks (duplicate state ids/symbols are errors, fewer styles than states is a warning).
  - `internal/cli`: flag parsing with the stdlib `flag` package (no CLI dependency, to keep WASM small) and `Run`.
  - `internal/document`: implemented. goldmark-based parser producing the syntax (name, META/BASE/STATE directives,
    list items as code/text tokens) with positions. Handles name/summary rules, marker errors, misplaced directives
    (code spans are masked, so docs that show the syntax in backticks are fine), duplicate blocks, empty states,
    missing titles, and non-list content (`invalid-range` in STATE, `meta-schema` in META, `unexpected-content` in BASE).
  - `internal/expr`: implemented. Parses one STATE item into a `Range` (start/end minutes, weekday list with wrap-around
    expanded). Item shape must be code, ` to `, code, whitespace plus weekdays (`invalid-range` otherwise). Times
    `H:MM`/`HH:MM` (`invalid-time`), multiples of `--sample-interval` (`sample-interval`), weekdays per the decided rules
    (`invalid-weekday` at the offending word, `-` or `,`).
  - `internal/schedule`: META validation against the embedded schema (schema errors mapped to the key or value
    position), state and base resolution, range parsing via `expr`, and painting (`paint.go`: every minute of the week
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
- **Help and generated reference (decided 2026-10-25):** the flag set's usage strings (`newFlagSet` in
  `internal/cli`) and `diag.Codes` are the single source for `--help` (`cli.Help`) and for the README's flag tables
  and diagnostic-code table, which sit between `<!-- generated: … -->` markers. `TestReadmeReference` fails when they
  are stale; `mise run docs` regenerates them. `flagGroups` orders the flags and names their placeholders and
  defaults; tests require every registered flag there once and every `Code` constant in `diag.Codes` once. `-h`,
  `--help` (also with other arguments) print the help to stdout and exit 0; `--version` is a regular flag. revive's
  `exported` rule is on. Skipped on purpose: `Example` tests (everything is `internal/`) and a separate docs site.
- **Examples in messages and docs:** times sit on a 15-minute grid (`6:15`, `21:45`). Never use `23:59`: it reads
  like a recommended end of day, but the end of a range is exclusive and "until midnight" is written `00:00`. The
  `24:00` error says exactly that.
- Commits follow conventional style (`feat:`, `feat!:` for breaking changes, `docs:`, `chore:`).
- An earlier AI-assisted authoring skill was removed on purpose (`8f556d4`). Don't reintroduce it unasked.
