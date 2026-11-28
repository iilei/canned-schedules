# Error reports

`canned-schedules` never posts anything to GitHub or GitLab. It writes an error report that your CI job or bot can
use, in one of three shapes:

- `txt` (default): one `path:line:col: severity[code]: message` line per problem, as editors and CI annotators expect.
- `json`: `{"schema-version", "file", "summary": {"errors", "warnings"}, "diagnostics": [...]}`; each diagnostic has
  `severity`, `code`, `file`, `line`, `column`, a ready-made `location` (`path:line:col`), `message` (plain prose),
  an optional `example` (the exact Markdown to type, backticks included) and an `excerpt` (the source line with a
  caret under the column). Schema: [`schema/report.schema.json`](https://github.com/iilei/canned-schedules/blob/master/schema/report.schema.json).
- `--error-report-template markdown-gfm`: the JSON rendered with the built-in
  [`templates/markdown-gfm.tmpl`](https://github.com/iilei/canned-schedules/blob/master/templates/markdown-gfm.tmpl), ready to paste into an issue comment.

```sh
./canned-schedules --error-report-template markdown-gfm --error-output-file report.md \
  schedule.md
```

There is exactly one built-in template. For your own layout, write the JSON report and render it yourself, for
example with gomplate; the built-in template is a working starting point:

```sh
./canned-schedules --error-output-format json --error-output-file report.json schedule.md
gomplate -c .=report.json -f templates/markdown-gfm.tmpl
```

Paths are reported as you pass them; absolute paths inside your home directory are shortened to `~/…`, so reports
don't spell out your user name. In CI, pass paths relative to the repository root so annotations match the files (on
many runners the workspace lies inside the home directory).

Safety: the report file is written atomically (temporary file plus rename), so a planted symlink is replaced instead
of written through. Control characters and bidirectional formatting characters in paths and messages are escaped,
so a crafted file name can't forge log lines. In the Markdown, everything that comes from the document or the file
name sits inside a fenced block, where links, HTML, `@mentions` and `#references` don't render.

## Diagnostic codes

Every diagnostic carries one of these codes; the golden fixtures pin them, so they are part of the contract.

<!-- generated: codes (mise run docs; from diag.Codes in internal/diag) -->
| Code                      | Severity | Meaning                                                                                                                                                                                      |
| ------------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `missing-name`            | error    | The document has no H1 heading; the H1 names the schedule.                                                                                                                                   |
| `duplicate-name`          | error    | More than one H1 heading.                                                                                                                                                                    |
| `invalid-name`            | error    | The name breaks the text rule: 1-80 ASCII letters, digits, spaces or hyphens, starting with a letter and ending with a letter or digit.                                                      |
| `invalid-summary`         | error    | The summary after the META marker breaks the same text rule as the name.                                                                                                                     |
| `invalid-marker`          | error    | A directive separates namespace and kind with `:` (`[!SCHEDULE:STATE]`); the separator is `-`.                                                                                               |
| `missing-kind`            | error    | A marker has the namespace but no kind (`[!SCHEDULE]`, `[!SCHEDULE-]`).                                                                                                                      |
| `unknown-kind`            | error    | A kind other than META, BASE or STATE.                                                                                                                                                       |
| `missing-title`           | error    | A BASE or STATE marker without a state name after it.                                                                                                                                        |
| `duplicate-block`         | error    | A second META or BASE block.                                                                                                                                                                 |
| `empty-state`             | error    | A STATE block without ranges.                                                                                                                                                                |
| `misplaced-directive`     | error    | A directive that isn't a top-level quote starting with its marker: nested in a list, wrapped in bold, or glued to the block above (no empty line between them, or a stray line without `>`). |
| `unexpected-content`      | error    | Content in a BASE block, which takes only a state name on the marker line.                                                                                                                   |
| `disallowed-alert`        | error    | An alert that doesn't match `--error-unless-alert-pattern`.                                                                                                                                  |
| `invalid-range`           | error    | A STATE item that isn't `` `start` to `end` WEEKDAYS ``, or content other than a list in a STATE block.                                                                                      |
| `invalid-time`            | error    | A time that isn't `H:MM` or `HH:MM` from `00:00` to `23:59` (`24:00` is rejected).                                                                                                           |
| `sample-interval`         | error    | A time that isn't a multiple of `--sample-interval`.                                                                                                                                         |
| `invalid-weekday`         | error    | A weekday expression with an unknown or lowercase name, or a broken range or list.                                                                                                           |
| `duplicate-weekday`       | error    | The same weekday written twice in one item (`MON,MON`, `WED-WED`).                                                                                                                           |
| `repeated-weekday`        | warning  | A day that an earlier part of the same item already covers (`MON-WED,TUE`); the range still applies.                                                                                         |
| `meta-schema`             | error    | META content that isn't a list of ``key: `value` `` items, or a key or value the META schema rejects.                                                                                        |
| `unknown-state`           | error    | A state name the configuration doesn't define, in BASE, STATE or `--base`.                                                                                                                   |
| `missing-base`            | error    | `--require-base true`, and the document has no BASE block.                                                                                                                                   |
| `config-schema`           | error    | The configuration, after overrides, fails its JSON Schema.                                                                                                                                   |
| `invalid-override`        | error    | A `--set-config-at` path that doesn't exist, or a `--set-config-value` that isn't JSON.                                                                                                      |
| `config-duplicate-state`  | error    | Two state ids in the configuration that are equal ignoring case.                                                                                                                             |
| `config-duplicate-symbol` | warning  | Two styles share a symbol, so the `styles` ASCII preset tells those states apart only by colour.                                                                                             |
| `style-reuse`             | warning  | Fewer styles than states: styles repeat round robin, so states become hard to tell apart.                                                                                                    |
| `invalid-flag`            | error    | An unknown flag or an invalid flag value.                                                                                                                                                    |
| `invalid-args`            | error    | No input file, several files, or flags after the file (exit code 2).                                                                                                                         |
| `input-file`              | error    | The input can't be read: missing, not a regular file, or larger than 256 KiB (exit code 2).                                                                                                  |
| `preview-approximate`     | warning  | Preview buckets mix states, so the preview rounds; reported only when a preview is requested.                                                                                                |
| `output-schema`           | error    | The tool's own output failed its schema: a bug in canned-schedules, please report it.                                                                                                        |
<!-- end generated -->
