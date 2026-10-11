---
name: contribute-schedule
description: "Use when contributing a new capacity schedule preset: create an issue proposal, validate with mise, generate Markdown and SVG previews and a Git patch, then prepare a reviewed GitLab merge request or GitHub pull request."
argument-hint: "Describe the new preset name and weekly pattern, or provide a proposal file."
---

# Contribute a schedule

Guide the user from a schedule idea to a validated contribution. Orchestrate the
repository's existing mise tasks; do not reimplement extraction, validation,
rendering, or patch generation. Run commands from the repository root.
Mise setup is covered separately in the handbook.

## 1. Inspect and clarify

- Inspect the working tree, current branch, and remotes. Preserve unrelated
  changes and never stage them as part of the contribution.
- Read `mise.toml`, `schedules.cue`, and the proposal examples in `demo/`.
  Use task help for current arguments rather than guessing options.
- Before suggesting any new preset name, obtain the current catalog names:

  ```sh
  mise exec -- sh -c 'cue export . -e capacity_schedules | jq -r "keys[]"'
  ```

  Present existing presets only as context or options to reuse, never as names
  for a new contribution. Check every suggested or user-provided name against
  this list before authoring a proposal. If the name exists, explain the
  collision and ask for a new name, or offer to use the existing preset without
  creating a contribution. Do not rely on a later validation failure to discover
  collisions. If catalog export fails, resolve or report it before suggesting
  names.
- Ask for missing information: a unique preset name, the MON-SUN pattern, and
  tier or day-helper choices. Ask one focused question at a time.
- Use the existing tiers and validation contract. ASG network configuration is
  outside this catalog; do not invent internal/external schedule modes.
- If the user supplies a proposal file, inspect it rather than replacing it.
  For a new file, use a descriptive Markdown filename in a user-agreed location.

## 2. Author the proposal

Follow the existing examples: YAML frontmatter at the start, bounded by `---`.
Require `name` and exactly one of:

- `schedule_visual`: seven unlabelled rows ordered MON-SUN.
- `schedule_cue`: a MON-SUN map of CUE expression strings using supported day
  helpers or slot expressions.

Optional metadata is `description`, `team`, and `owner` (a list of strings).
Names start with a lowercase letter and use lowercase letters, digits, and
underscores. Put any explanatory context after the frontmatter. Let the existing
tasks enforce the input contract; do not add unsupported keys or expressions.

## 3. Validate, preview, and request review

For example, with a proposal at `demo/new-preset.md`:

```sh
mise run vet-proposal demo/new-preset.md
mise run preview-proposal demo/new-preset.md --svg
```

The first task validates; the second also validates and generates previews and
a patch. They take the file as a positional argument, not `--file`.
If regeneration is needed, use `--force` only for known generated outputs.
It does not bypass duplicate-name validation.

Show links to the generated files beside the proposal:

- `<input-stem>.generated-text.md`
- `<input-stem>.generated-img.svg`
- `<input-stem>.generated.patch`

Also offer "Open SVG preview in browser", not just an editor-file link.
Verify that the generated SVG exists and resolve its absolute path. If browser
tools are available, open the SVG using its properly encoded `file:///` URL and
share the browser link returned by the tool, if provided. Otherwise, provide the
local browser URL for the user to copy into their browser. Do not claim that an
ordinary workspace-file link opens a browser, or invent a browser link.
Keep the preview local; do not upload it or start a server just to show it.

Summarize the weekly pattern and derived UTC cron transitions. Explain that
validation checks structural plausibility, not operational suitability.
Ask the user to approve the preview before applying the patch.
For an issue-only contribution, stop here and offer the proposal, preview, and
patch as attachments; publish an issue only with user approval.

## 4. Prepare the catalog change

After preview approval, create a contribution branch, unless the user already
has a suitable branch. If unrelated work makes branching or staging ambiguous,
ask before proceeding. Do not reset, stash, or discard user changes.

```sh
git switch -c feat/schedule-new-preset
git apply --check demo/new-preset.generated.patch
git apply demo/new-preset.generated.patch
mise run fmt
mise exec -- sh -c 'cue export . > test.cue.json'
mise run check
```

Adapt branch and proposal names to the actual contribution.
The patch adds an explicit preset at the end of `capacity_schedules` in
`schedules.cue`, immediately before `...`; it leaves `_base_schedules` unchanged.
Inspect the patch before applying it. If it is stale, regenerate and review it
rather than forcing application.

After applying, use catalog checks and normal rendering tasks. Do not run
`vet-proposal` or `preview-proposal` for the now-present name: they correctly
reject duplicates. For updated catalog previews, use `render-time-table` or
`render-schedule-svg` without `--proposal`.

Inspect formatting and snapshot diffs for unrelated changes. Follow the current
repository policy for checked-in generated documentation. Never edit expected
results merely to hide a failure. Fix contribution-caused failures; report
unrelated failures and request guidance instead of silently changing them.
Do not claim the contribution is ready to merge unless the checks pass.

## 5. Review, commit, and publish

- Review the full contribution diff and report validation results.
- Ask for approval before committing, pushing, or creating a merge request.
- Stage only the agreed contribution files, using explicit paths. Generated
  previews and patches are ignored and are attachments, not files to force-add.
  Include the authored proposal only if the user wants it retained in the repo.
- Commit with a descriptive message and any required repository trailers.
- Inspect the chosen remote and hosting CLI authentication before publishing.
  Do not assume GitLab: this repository may also be hosted on GitHub.
  Use `glab` for GitLab or `gh` for GitHub when available. If the target remote,
  base branch, or publication permissions are unclear, ask the user.
- Push only the contribution branch; never force-push.
- Create the merge request or pull request with the preset purpose, weekly
  pattern, validation results, and relevant issue reference or preview.
  If no suitable CLI is available, give the branch and web creation link and
  state that publication is still pending.
- Finish with the request link and any remaining blockers. Publication is not
  complete until confirmed by the hosting service.
