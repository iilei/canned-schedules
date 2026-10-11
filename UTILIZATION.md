# Utilization and Enterprise Distribution Flow

## Purpose

This repository is designed to act as the source of truth for reusable capacity schedules that can be reviewed, versioned, published, and consumed by teams across an organization.

The core idea is simple:

- define a schedule as CUE,
- validate it with the repo’s contract checks,
- render it visually as tables and SVGs,
- publish the generated artifacts to a shareable distribution channel,
- and make the available presets easy to inspect and choose from.

In an enterprise context, this becomes a catalog of “approved” operating capacity patterns for internal services and platforms.

## Target operating model

### 1. The repo is the canonical schedule catalog

`canned-schedules` remains its own repository, with ownership over:

- schedule definitions,
- validation rules,
- rendering templates,
- generated previews,
- exported artifacts for sharing.

This keeps the schedule library independent from a specific application or handbook repo.
It also makes it possible to grant curated access to people who need to review or consume the published outputs, without requiring them to read the source CUE directly.

### 2. The rendered output is visible to humans

The repo should expose a generated `output/` bundle that contains:

- Markdown tables,
- SVG visuals,
- JSON exports,
- and any generated schedule metadata.

These files should be easy to browse in GitLab or in a release artifact view. For teams operating from a web UI, the generated output is a product-like preview: “here are the approved presets we have available.”

### 3. The docs repo can assemble these presets into a handbook

The Sphinx handbook repo becomes the consumer-facing documentation layer. A docs build can pull the generated output from `canned-schedules` or from an artifact publication target and embed it into the handbook.

This lets the docs pipeline show:

- the presets list,
- a schedule summary,
- per-schedule visual examples,
- and links to the versioned artifacts.

The overall flow is:

- schedule definition in `canned-schedules`
- CI render and validation
- publication to Artifactory
- Sphinx handbook embeds the generated result

## Recommended repository flow

### Source of truth

The source repo contains the validated CUE schedule definitions and reusable rules.

Example structure:

- `schedules.cue` — authored presets and generated export bundle
- `schedule-core.cue` — shared model and validation logic
- `schedule-schema.cue` — schema constraints and contract checks
- `render.sh` — render pipeline for tables/SVGs
- `output/` — generated artifacts (versioned or CI-produced)

### Human review

The generated output should be readable by anyone with access to the repo or the published artifact set.

Examples of generated assets:

- `output/SCHEDULES.md`
- `output/schedule-standard.svg`
- `output/schedule-weekday_maximum.svg`
- `output/schedule.json`

These should be straightforward to browse and compare.

### Contribution model

If someone wants to add a new preset, they should be able to contribute in either of two ways:

1. Submit a CUE definition in a merge request.
2. Open an issue with either:
   - an ASCII-art schedule, or
   - a CUE snippet,
   and let maintainers convert it into the canonical config.

This matters because the repo is intended to support reuse, not only internal authoring. A contributor can provide the schedule in a format that is easy to understand and then convert it into a validated configuration.

A useful issue template would ask for:

- schedule name,
- intended use case,
- weekday-by-day pattern,
- optional ASCII-art representation,
- desired tier mapping,
- and operational requirements for the intended Auto Scaling Group (ASG).

This makes the human review process easy and keeps the repo’s “ground truth” as structured CUE data.

### Local proposal validation and previews

The contribution entry point is exposed through mise tasks:

```sh
mise run vet-proposal demo/a_issue_example_visually.md
mise run preview-proposal demo/b_issue_example_cue.md > preview.md
```

Both example documents use strict YAML frontmatter. A proposal provides either
seven visual day strings in MON-SUN order or a `schedule_cue` weekday map using
shared helpers and custom slot expressions. The task extracts only the
frontmatter, validates an isolated candidate with CUE, and leaves the catalog
unchanged.

`preview-proposal` reuses `render-time-table` with Markdown, rulers, a legend,
and UTC cron transitions. Contributors can also invoke `render-time-table
--proposal <file>` directly to select rendering options, or request SVG previews.
The preview bundle includes candidate CUE source and exported JSON for review.
See [README.md](./README.md#previewing-a-proposed-preset) for the input contract
and commands.

The issue body after the frontmatter is optional context. Users may include the
generated visual preview there, attach it, or omit it entirely. Automation must
read the frontmatter rather than reconstruct a schedule from incidental examples
in the issue body. Maintainers review and promote the validated candidate through
a merge request; previewing does not apply a patch or publish a release.

## CI/CD workflow

### Validation stage

On every merge request or tag:

- run `cue vet .`
- export the JSON representation,
- compare against expected snapshots,
- render the schedule visuals,
- and verify the generated artifacts are structurally valid.

This is the same pattern already modeled by `tests/check.sh` in this repo.

### Render stage

The CI job should render the full bundle:

- Markdown summary tables,
- per-schedule SVGs,
- exported `schedule.json`,
- and any ancillary output used by docs.

The build target should be a consistent output directory such as:

- `output/`

### Publication stage

Once checks pass, the CI pipeline publishes the rendered bundle to Artifactory in a versioned path, for example:

- `releases/canned-schedules/v0.0.1/`
- or a more structured namespace such as:
  - `capacity-schedules/canned/v0.0.1/`

The published set should include:

- `schedule.json`
- `SCHEDULES.md`
- SVG files
- optional manifest metadata

This makes the artifacts suitable for:

- repo browsing,
- docs integration,
- service-level dependency references,
- and release tracking.

## Integration with the Sphinx docs repo

The Sphinx handbook should not be the source of truth. Instead, it should read the generated artifacts from the schedule repo or a published artifact source.

### Example pattern

The handbook pipeline can:

1. fetch the published schedule bundle,
2. read the list of presets,
3. render a docs page per schedule,
4. embed the SVG or Markdown preview,
5. and link each preset to a stable artifact path.

This makes the docs repo an index and presentation layer while the `canned-schedules` repo remains the authoritative source.

A docs page could look like:

- “Available capacity presets”
- “weekday_maximum”
- “all_medium”
- “standard”
- “minimum”
- “turned_off”

Each preset entry includes:

- a short description,
- a visual preview,
- the schedule JSON export path,
- the version tag,
- and any operational notes.

## Example consumption contract

Once this is in place, consumers can refer to schedule presets by version and use-case.

An example intent might be:

- “I want canned-schedules v0.0.1 `weekday_maximum`, with its `capacity_tiers`, applied to the ASGs for service `xyz`.”

This can map cleanly to a governed catalog entry:

- repo: `canned-schedules`
- version: `v0.0.1`
- preset: `weekday_maximum`
- service: `xyz`
- targets: the ASGs serving `xyz`

The schedule itself is selected by version and name. Its tier values provide the
ASG minimum, desired, and maximum capacity settings at each scheduled transition.
The consuming infrastructure configuration decides which ASGs receive the
schedule and how those groups are networked.

Internal or external refers to the ASGs' network configuration, not to separate
schedule modes or capacity profiles. The same preset can be applied to ASGs in
either network configuration; network settings remain outside the schedule
catalog.

This is a strong governance pattern: it is not “just a diagram”, it is a versioned system capability with explicit names and publication points.

## Operational value

This model yields several benefits:

- easy review of approved schedule presets,
- deterministic rendering and export,
- versioned artifacts with traceable provenance,
- central catalog for internal standards,
- clear contribution path for new schedule ideas,
- and clean integration with handbook and automation workflows.

## Recommended end state

The target shape is:

- `canned-schedules` repository is the source and distribution point,
- GitLab CI validates and renders the content,
- Artifactory stores the published outputs with stable versioning,
- handbook docs consume the generated bundle,
- and internal teams select approved presets by name and version rather than hand-authoring ad hoc schedule data.

This gives the repo a durable “catalog of schedule presets” role in the enterprise, while keeping the technical implementation compact and reviewable.

## Summary

This repo is not just a rendering utility. It can become a governed library of reusable curation patterns for operational capacity planning.

The natural enterprise flow is:

- store the canonical preset definitions in a dedicated repo,
- render and validate them in CI,
- publish them to Artifactory,
- embed the generated assets in the Sphinx handbook,
- and make the approved schedules visible and easy to select by version.
