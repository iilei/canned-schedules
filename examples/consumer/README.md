# Canned schedules in your repo

A ready-to-copy setup for a repo that keeps approved schedules as `*.canned-schedule.md` files and checks schedule
requests that arrive as issues. Everything runs through [mise](https://mise.jdx.dev) tasks, so CI and your machine use
the same commands. This folder is tested in the canned-schedules CI (`mise run example:test`).

| File | What it does |
| --- | --- |
| `mise.toml` | Pins the canned-schedules release and defines the `schedule:*` tasks |
| `scripts/canned-schedule` | The tasks themselves (POSIX `sh`); runs without mise too |
| `canned-schedules.args` | Flags for every call, one per line |
| `.gitignore` | Keeps the generated files out of git |
| `schedules/office-hours.canned-schedule.md` | An example schedule (the issue template's example) |
| `.github/ISSUE_TEMPLATE/canned_schedule_request.md` | The issue template people propose schedules with |
| `.github/workflows/schedule-request.yml` | Checks request issues and keeps one comment with the result |

## Set up

1. Copy the files into your repo (merge `mise.toml` and `.gitignore` if you have them already) and run `mise install`.
2. Install the pre-commit hook: `mise generate git-pre-commit --write --task schedule:pre-commit`.
3. Put schedules under `schedules/` as `<name>.canned-schedule.md`. The `.gitignore` patterns cover that folder only;
   adjust them if you keep schedules elsewhere.

## Tasks

- `mise run schedule:render [file...]`: renders every schedule git doesn't ignore (or the given files) into
  `.canned-schedule.json` (the canonical output), `.canned-schedule.svg` (the preview, with the request embedded) and
  `.canned-schedule.txt` (the text form: the canonical request with the ASCII preview). It prints the text form, in
  colour on a terminal, reports every broken file before it fails, and deletes outputs that would otherwise be stale:
  those of a file that failed, and those whose source is gone. Files git tracks are never deleted.
- `mise run schedule:check <file> [out-dir]`: checks one request, for example an issue body, and writes
  `report.md`, `report.json`, `comment.md` and, for a valid request, `schedule.json` and `schedule.txt` into `out-dir`
  (default `canned-schedule-report`, gitignored). `comment.md` is what the workflow posts.
- `mise run schedule:pre-commit`: checks the staged content of every staged schedule, not the working tree, and names
  each problem by its path in the repo.

None of the tasks talks to GitHub or GitLab. The workflow posts the comment; the task only writes files.

## Settings

- **Flags:** `canned-schedules.args` lists arguments for every call, one per line (blank lines and lines starting
  with `#` are skipped). The example allows only GitHub's standard alerts next to the schedule directives, so a typo
  such as `[!SCHEDUEL-STATE]` fails instead of being read as prose. Your own states go here too:

  ```text
  --set-config-at
  states
  --set-config-value
  [{"id": "Nobody", "display-name": "Nobody"}, {"id": "Team-EU", "display-name": "Team EU"}]
  ```

- **Binary:** `CANNED_SCHEDULES` overrides the binary (default `canned-schedules` on `PATH`), and
  `CANNED_SCHEDULES_ARGS` another args file.
- **Version:** bump the release in `mise.toml` deliberately. A new version can reject what an old one accepted, and
  the issue template, the hook and CI should agree on one.

## Committed previews

GitHub only shows an SVG that is committed. To show one, for example in a README, remove its pattern from
`.gitignore`, commit the file, and let CI run `mise run schedule:render` followed by `git diff --exit-code`, so the
committed preview can't drift from its source.
