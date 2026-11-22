# Schedules in a repo

Keep each approved schedule as a `*.canned-schedule.md` file, for example `schedules/on-call.canned-schedule.md`. The
suffix lets tools find them without catching unrelated Markdown. [`examples/consumer`](https://github.com/iilei/canned-schedules/tree/master/examples/consumer) is a
ready-to-copy setup, tested in this repo's CI:

- **`mise run schedule:render [file...]`** renders every schedule (or the given ones) into three files next to it and
  prints the text form, in colour on a terminal:
  - `.canned-schedule.json`: the canonical output;
  - `.canned-schedule.svg`: the preview, with the schedule embedded;
  - `.canned-schedule.txt`: the text form.

  It reports every broken file before failing. When a file fails, its outputs are deleted, and so are outputs whose
  source is gone, so nothing stale looks current. Files git tracks are never deleted.
- **The outputs are gitignored** by the example's `.gitignore`. To show a preview on GitHub, commit that SVG instead
  and let CI render and run `git diff --exit-code`.
- **`mise run schedule:check <file> [out-dir]`** checks one request, for example an issue body, and writes
  `report.md`, `report.json`, `comment.md` (the comment to post) and, when valid, the JSON, the text form and the
  SVG. The task itself never talks to GitHub. The example's issue workflow runs the same check with the
  [canned-schedules-companion](https://github.com/iilei/canned-schedules-companion) action, which uploads the files as
  workflow artifacts and posts the comment with links to them.
- **`mise run schedule:pre-commit`** checks what is staged, not the working tree. Install it as a git hook with
  `mise generate git-pre-commit --write --task schedule:pre-commit`.
- **`canned-schedules.args`** holds flags for every call, one per line. The example makes alerts strict, so a typo
  such as `[!SCHEDUEL-STATE]` fails instead of being read as prose.

Without mise, the hooks also work with [pre-commit](https://pre-commit.com), which builds the binary from the tag:

```yaml
repos:
  - repo: https://github.com/iilei/canned-schedules
    rev: v0.3.4
    hooks:
      - id: canned-schedules
```
