# Canned Schedules

Weekly capacity schedules from human-friendly Markdown: validate, preview and export.

Teams propose a schedule in a GitHub or GitLab issue, written in plain Markdown. `canned-schedules` checks it, points
at mistakes by line and column, and turns it into canonical JSON: the state in effect at every minute of the week, plus
the moments the state changes, with ready-made cron expressions.

> [!NOTE]
> Work in progress. The CLI, the input syntax, error reports and previews work end to end; releases are still to
> come.

## Example

```markdown
# Office Hours

> [!SCHEDULE:META] Weekdays with a late Friday
> - x-owner: `team-xyz`

> [!SCHEDULE:BASE] Minimum

> [!SCHEDULE:STATE] Medium
> - `06:00` to `22:00` MON-FRI
> - `10:00` to `18:00` SAT

> [!SCHEDULE:STATE] Maximum
> - `16:00` to `00:00` FRI
```

![Preview of the Office Hours schedule](docs/office-hours.svg)

Read it as: the base state is `Minimum`. `Medium` applies on weekdays from 06:00 to 22:00 and on Saturdays from 10:00
to 18:00. `Maximum` applies on Fridays from 16:00 until midnight; it comes later, so it wins where it overlaps the
Friday `Medium` range.

Start a request from the issue template: [GitHub](.github/ISSUE_TEMPLATE/canned_schedule_request.md),
[GitLab](.gitlab/issue_templates/canned_schedule_request.md).

## Try it in the browser

The **preflight page** runs the same checker, compiled to WebAssembly, entirely in your browser: start from a preset
(from a bare "Medium all week" to the full issue template), edit it in a highlighting editor that marks errors and
warnings on their lines, and see the week as SVG, the issue as GitHub renders it, a Sphinx handbook view with admonitions coloured from the state
styles (downloadable as a [Sphinx kit](#sphinx)), diagnostics that jump to the offending line, the JSON and the ASCII preview. Light and dark themes included;
nothing is uploaded. It is published to GitHub Pages from `master`; locally:

```sh
mise run web:serve      # builds dist/site and serves it on http://127.0.0.1:8080 (--port to change)
```

## Syntax

A schedule is an H1 heading (the name) plus directive blocks: quotes that start with a `[!SCHEDULE:…]` marker.
Everything else is free text for humans, including standard alerts such as `[!NOTE]`.

| Block                     | Required      | Content                                                                                     |
| ------------------------- | ------------- | ------------------------------------------------------------------------------------------- |
| `[!SCHEDULE:META] hint`   | no            | Optional hint after the marker; list items `` x-<name>: `value` `` for your own bookkeeping |
| `[!SCHEDULE:BASE] state`  | no            | The state wherever no range applies                                                         |
| `[!SCHEDULE:STATE] state` | one per state | A list of ranges                                                                            |

`SCHEDULE` is the only namespace. Markers and state names are case-insensitive.

**Ranges** are written `` `start` to `end` WEEKDAYS ``:

- Times are `H:MM` or `HH:MM`, e.g. `6:15` or `21:45`.
- The end is exclusive: `06:00` to `22:00` covers 06:00 up to, but not including, 22:00.
- A range that runs until midnight ends at `00:00`. There is no `24:00`.
- A range belongs to its start day and may cross midnight: `22:00` to `02:00` FRI runs into Saturday 02:00.
- Equal start and end means a full 24 hours: `08:00` to `08:00` MON runs until Tuesday 08:00.
- Weekdays are `MON TUE WED THU FRI SAT SUN` in uppercase, with ranges (`MON-FRI`, also across the weekend: `FRI-MON`)
  and lists (`SAT,SUN`). Weekday ranges are **inclusive**: `MON-FRI` includes Friday, unlike the time range, whose end
  is exclusive.
- Each weekday is written at most once per range: `MON,MON` and `WED-WED` are errors (write `WED`). A day that an
  earlier range already covers, as in `MON-WED,TUE`, gets a warning: it's harmless but usually a typo.
- Later ranges win where they overlap earlier ones.

The states come from the configuration: `Off`, `Minimum`, `Medium` and `Maximum` by default. A state is anything that
holds for a stretch of the week, such as a capacity level, a support team or a maintenance mode. To use your own, see
[Your own states](#your-own-states).

## Usage

Build from a checkout (Go 1.25, tools pinned via [mise](https://mise.jdx.dev)):

```sh
mise install
go build ./cmd/canned-schedules
./canned-schedules [flags] request.md
```

The input is always a regular file of at most 256 KiB, given after the flags. On success the canonical JSON goes to
stdout and the exit code is 0. Otherwise each problem goes to stderr as `path:line:col`, with exit code 1 for errors in
the document or in flag values and 2 for an unusable command line or environment (no file, several files, flags after
the file, unreadable input, report not writable):

```text
request.md:9:17: error[invalid-time]: 24:00 is not a valid time: the end of a range is exclusive, so a range that runs until midnight ends at 00:00, e.g. `21:45` to `00:00`
```

| Flag                          | Default     | Meaning                                                                         |
| ----------------------------- | ----------- | ------------------------------------------------------------------------------- |
| `--sample-interval <minutes>` | `1`         | Every time must be a multiple of it; must divide 60 (e.g. `15`, `60`)           |
| `--require-base true\|false`  | `false`     | `true`: the document must contain a BASE block; can't be combined with `--base` |
| `--base <state>`              | first state | Base state when the document has no BASE block                                  |
| `--set-config-at <path>`      |             | Config path to replace, e.g. `representation.styles.states[0]` (repeatable)     |
| `--set-config-value <json>`   |             | JSON value for the matching `--set-config-at` (repeatable)                      |
| `--output json\|ascii`        | `json`      | What stdout carries: the canonical JSON or the ASCII preview                    |
| `--svg-output-file <path>`    |             | Write an SVG preview (only when the document has no errors)                     |
| `--svg-theme <theme>`         | `auto`      | SVG colours: `auto` (follow the viewer), `light` or `dark`                      |
| `--ascii-output-file <path>`  |             | Write an ASCII preview (only when the document has no errors)                   |
| `--myst-output-file <path>`   |             | Write the schedule as MyST for Sphinx (only when the document has no errors)    |
| `--sphinx-kit-output-file`    |             | Write the Sphinx kit zip (only when the document has no errors)                 |
| `--preview-downsample[=N]`    | 15          | Preview buckets in minutes (must divide 60); `=N` needs the `=`                 |
| `--no-preview-downsample`     |             | Preview at 1-minute resolution (1440 columns per day)                           |
| `--ascii-symbols <preset>`    | `ascii`     | Symbols in the ASCII preview: `ascii`, `braille` or `styles`                    |
| `--color auto\|always\|never` | `auto`      | Colours in the ASCII preview; auto = terminal only, and not with `NO_COLOR`     |
| `--error-output-file <path>`  | stderr      | Write the error report to this file (always, even when empty)                   |
| `--error-output-format`       | `txt`       | Error report format: `txt` or `json`                                            |
| `--error-report-template`     |             | Render the error report with the built-in `markdown-gfm` template instead       |
| `--version`                   |             | Print the version                                                               |

## Previews

```sh
./canned-schedules --output ascii request.md
```

```text
Office Hours - Weekdays with a late Friday

    00:00                   06:00                   12:00                   18:00
    + : | : | : | : | : | : + : | : | : | : | : | : + : | : | : | : | : | : + : | : | : | : | : | :
MON ::::::::::::::::::::::::================================================================::::::::
FRI ::::::::::::::::::::::::========================================################################
SAT ::::::::::::::::::::::::::::::::::::::::================================::::::::::::::::::::::::

    . Off   : Minimum (base)   = Medium   # Maximum
```

(Rows for TUE–THU and SUN omitted here.) On a terminal each cell gets the dark theme's state colour as its background;
`--color never` turns that off, `--color always` forces it (also into files).

**Symbols.** By default the preview is pure ASCII (`.` `:` `=` `#` by state order), so it lines up in every terminal,
log and email. `--ascii-symbols braille` switches to `⠀` `⣀` `⣤` `⣶` `⣿`, a denser look that is still single-width
everywhere (unlike block characters such as `▒`, which CJK terminals draw double-width). `--ascii-symbols styles`
uses each configured style's own `symbol`. The flag is shorthand for
`--set-config-at representation.ascii.symbols`.

**Downsampling.** Previews show buckets of 15 minutes by default, so a day fits into 96 columns (100 with the day
label). `--preview-downsample=N` picks another bucket size (a divisor of 60), `--no-preview-downsample` shows every
minute. A bucket that contains more than one state shows the one covering most of it; the tool then warns that the
preview is approximate. Downsampling only affects previews: the JSON output stays exact. For exact previews, require
times on the bucket grid with `--sample-interval 15`.

### SVG preview

`--svg-output-file preview.svg` writes a picture of the week like the one above: one row per day, a time ruler, the
transitions with POSIX and AWS cron, and a legend with the base state marked. Hovering a block shows its state, its time
range and the cron expression that starts it. Colours come from the configuration's styles and follow the viewer's
light or dark mode. The preview is only written when the document has no errors, so a failed run never replaces a
good preview with a broken one.

The SVG comes from the built-in [`templates/schedule-svg.tmpl`](templates/schedule-svg.tmpl). The tool computes all
coordinates, colours and labels, so the template is plain markup that also runs with gomplate.

## Sphinx

The preflight page's Sphinx view is not a mock-up: the same look is available for real Sphinx docs built with the
[Furo](https://pradyunsg.me/furo/) theme. `--sphinx-kit-output-file kit.zip` (or **Download Sphinx kit** on the page)
writes a zip with:

- a local Sphinx extension (`_ext/canned_schedules`) that adds the stylesheet and fonts and enables MyST's
  `colon_fence`;
- the stylesheet, generated from the same state styles as the SVG and the page. It styles only schedule admonitions,
  through Furo's own admonition variables, and follows Furo's light, dark and auto modes;
- Ubuntu and Ubuntu Mono, used only inside schedule admonitions;
- the request as MyST, and a README with the two `conf.py` lines:

  ```python
  sys.path.append(os.path.abspath("_ext"))
  extensions += ["canned_schedules"]
  ```

`--myst-output-file schedule.md` writes just the MyST for a request: the name, the hint, and the META, BASE and STATE
blocks as `:::{admonition}` blocks with the classes the stylesheet expects, items written as in the request. The issue's
free text stays out. Colours follow `--set-config-*`; regenerate the kit after changing the state styles. On the page,
**Download MyST** gives the same file for the document in the editor.

### From request to docs page

Treat the MyST as build output: people edit and review the GitHub-flavoured request, and the docs build regenerates
the MyST from it. Nobody edits the MyST by hand. Validation then stays in one place, because the tool writes no MyST
for an invalid request and exits with 1.

In CI, run a step like this before `sphinx-build`:

```sh
for src in schedules/*/source.md; do
  name=$(basename "$(dirname "$src")")
  canned-schedules --myst-output-file "docs/schedules/$name.md" "$src" >/dev/null
done
```

- **Invalid schedules fail the build.** The error report goes to stderr; use `--error-output-file` to keep it.
- **Generated files:** either add `docs/schedules/` to `.gitignore`, or commit the files and let CI regenerate them
  and run `git diff --exit-code docs/schedules`, so they can't drift from the requests.
- **Each generated file is a page:** list them in a toctree, for example with `:glob:` and `schedules/*`. Put prose
  about a schedule on a hand-written page that links to it.

## Error reports

`canned-schedules` never posts anything to GitHub or GitLab. It writes an error report that your CI job or bot can
use, in one of three shapes:

- `txt` (default): one `path:line:col: severity[code]: message` line per problem, as editors and CI annotators expect.
- `json`: `{"schema-version", "file", "summary": {"errors", "warnings"}, "diagnostics": [...]}`; each diagnostic has
  `severity`, `code`, `file`, `line`, `column`, a ready-made `location` (`path:line:col`), `message` (plain prose),
  an optional `example` (the exact Markdown to type, backticks included) and an `excerpt` (the source line with a
  caret under the column). Schema: [`schema/report.schema.json`](schema/report.schema.json).
- `--error-report-template markdown-gfm`: the JSON rendered with the built-in
  [`templates/markdown-gfm.tmpl`](templates/markdown-gfm.tmpl), ready to paste into an issue comment.

```sh
./canned-schedules --error-report-template markdown-gfm --error-output-file report.md request.md
```

There is exactly one built-in template. For your own layout, write the JSON report and render it yourself, for
example with gomplate; the built-in template is a working starting point:

```sh
./canned-schedules --error-output-format json --error-output-file report.json request.md
gomplate -c .=report.json -f templates/markdown-gfm.tmpl
```

Paths are reported as you pass them; absolute paths inside your home directory are shortened to `~/…`, so reports
don't spell out your user name. In CI, pass paths relative to the repository root so annotations match the files (on
many runners the workspace lies inside the home directory).

Safety: the report file is written atomically (temporary file plus rename), so a planted symlink is replaced instead
of written through. Control characters and bidirectional formatting characters in paths and messages are escaped,
so a crafted file name can't forge log lines. In the Markdown, everything that comes from the document or the file
name sits inside a fenced block, where links, HTML, `@mentions` and `#references` don't render.

## Output

```json
{
  "schema-version": "1",
  "name": "Office Hours",
  "hint": "Weekdays with a late Friday",
  "base": "Minimum",
  "annotations": {"x-owner": "team-xyz"},
  "digests": {"schedule": "sha256:…"},
  "week": {
    "MON": [
      {"start": "00:00", "end": "06:00", "state": "Minimum"},
      {"start": "06:00", "end": "22:00", "state": "Medium"},
      {"start": "22:00", "end": "00:00", "state": "Minimum"}
    ],
    "TUE": ["…"]
  },
  "transitions": [
    {
      "time": "06:00",
      "days": ["MON", "TUE", "WED", "THU", "FRI"],
      "state": "Medium",
      "cron": {"posix": "0 6 * * 1-5", "aws": "0 6 ? * MON-FRI *"}
    }
  ],
  "warnings": []
}
```

- `week` covers every day from `00:00` to `00:00` in half-open runs; an `end` of `00:00` means the end of the day.
- `transitions` lists every moment the state changes, grouped by time and state. `cron.posix` is five-field Unix cron
  (as EC2 Auto Scaling scheduled actions use), `cron.aws` the six-field EventBridge format. Neither carries a time
  zone: set it where you schedule them.
- `warnings` lists warnings (e.g. `style-reuse`, `preview-approximate`) with code, message and, when positioned,
  line, column and excerpt. No file names: the output depends only on the document and the flags.
- `digests.schedule` changes only when the base or the week changes, never for the name, hint, annotations or
  prose. To trace a schedule back to its source, carry a git SHA or an issue link in an `x-*` META key.
- The schema is [`schema/output.schema.json`](schema/output.schema.json); details in
  [`testdata/golden/README.md`](testdata/golden/README.md).

Provider settings such as capacity numbers are out of scope: attach them to the state ids downstream.

## Configuration

The built-in configuration ([`schema/config.json`](schema/config.json), validated by
[`schema/config.schema.json`](schema/config.schema.json)) defines the ordered state list and how states look: page
colours per light/dark theme, one style (symbol and colours) per state assigned round robin, and the symbol preset of
the ASCII preview (`representation.ascii.symbols`). Override any part of
it per run:

```sh
./canned-schedules \
  --set-config-at 'states[0].display-name' --set-config-value '"Switched off"' \
  request.md
```

An override replaces the value at its path; the path and the array index must exist.

### Your own states

Replace the whole `states` list to schedule something other than capacity. For example, follow-the-sun on-call:

```sh
./canned-schedules \
  --set-config-at states \
  --set-config-value '[{"id": "Nobody",  "display-name": "Nobody"},
                       {"id": "Team-EU", "display-name": "Team EU"},
                       {"id": "Team-US", "display-name": "Team US"}]' \
  oncall.md
```

```markdown
# On-Call

> [!SCHEDULE:BASE] Nobody

> [!SCHEDULE:STATE] Team-EU
> - `06:00` to `14:00` MON-FRI

> [!SCHEDULE:STATE] Team-US
> - `14:00` to `22:00` MON-FRI
```

- Only the listed ids are valid in BASE and STATE blocks. The first one is the base when the document has none.
- Colours and symbols are assigned round robin from `representation.styles.states`, so override those only for
  specific colours.
- To share a configuration, keep it in a file and pass `--set-config-value "$(cat states.json)"`.
- The week repeats unchanged. Rotations that alternate between weeks ("Alice this week, Bob next week") can't be
  expressed, and overlapping roles such as primary and secondary on-call need one document each.

## Development

```sh
mise run check      # format check, vet, lint, tests (race detector), build
mise run test:wasm  # the same tests compiled to WebAssembly, run under Node
mise run web:serve  # build and serve the preflight page
mise run fmt        # apply formatting
```

The spec lives in [`testdata/golden`](testdata/golden): one directory per case with the input, optional CLI
arguments, and the expected output or diagnostics. `python3 testdata/golden/gen.py` regenerates them from an
independent reference implementation; review the diff like any other change. Never derive fixtures from the Go code.

The page's editor is [CodeMirror 6](https://codemirror.net/), bundled from [`editor/`](editor) by `web:build` with the
versions locked in `editor/package-lock.json` (`npm ci`, install scripts off). The licences of the bundled packages
are published next to it as `editor-licenses.txt`. Without `editor.js` the page falls back to a plain textarea.
