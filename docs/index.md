# Canned Schedules

Weekly schedules that humans and machines agree on.

**[Open the playground](https://iilei.github.io/canned-schedules/playground/)** to write a schedule and see its week
right away; nothing leaves your browser.

Teams propose a schedule in a GitHub or GitLab issue, written in plain Markdown. `canned-schedules` checks it, points
at mistakes by line and column, and turns it into canonical JSON: the state in effect at every minute of the week, plus
the moments the state changes, with ready-made cron expressions.

:::{note}
Version 0.x: the CLI, the input syntax, error reports and previews work end to end, and
[releases](https://github.com/iilei/canned-schedules/releases) are signed. The contract may still change before 1.0;
see [Stability](stability.md) for how changes are announced.
:::

## Example

```markdown
# Office Hours

> [!SCHEDULE-META] Weekdays only
> - x-owner: `team-xyz`

> [!SCHEDULE-BASE] Off

> [!SCHEDULE-STATE] Medium
> - `08:00` to `18:00` MON-FRI
```

:::{image} _generated/office-hours-light.svg
:alt: Preview of the Office Hours schedule
:class: only-light
:::

:::{image} _generated/office-hours-dark.svg
:alt: Preview of the Office Hours schedule
:class: only-dark
:::

Read it as: the base state is `Off`, and `Medium` applies on weekdays from 08:00 to 18:00.

Propose a schedule with one of three short issue templates, each showing one idea:

| Template     | Shows                                  | GitHub                                                                                                                  | GitLab                                                                                                                   |
| ------------ | -------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Office hours | A base state and one range             | [template](https://github.com/iilei/canned-schedules/blob/master/.github/ISSUE_TEMPLATE/canned_schedule_request.md)     | [template](https://github.com/iilei/canned-schedules/blob/master/.gitlab/issue_templates/canned_schedule_request.md)     |
| Night shift  | A range that crosses midnight          | [template](https://github.com/iilei/canned-schedules/blob/master/.github/ISSUE_TEMPLATE/canned_schedule_night_shift.md) | [template](https://github.com/iilei/canned-schedules/blob/master/.gitlab/issue_templates/canned_schedule_night_shift.md) |
| Friday peak  | Two states, where the later range wins | [template](https://github.com/iilei/canned-schedules/blob/master/.github/ISSUE_TEMPLATE/canned_schedule_friday_peak.md) | [template](https://github.com/iilei/canned-schedules/blob/master/.gitlab/issue_templates/canned_schedule_friday_peak.md) |

The bot labels an issue with a valid schedule `Schedule` and takes the label off when the schedule breaks; GitLab creates
the label when it is missing.

## How it fits together

Schedules are proposed as issues, kept in a catalog repo, published as versions, and consumed by pinned version:

```mermaid
flowchart TB
  subgraph propose["1 · Propose"]
    direction LR
    issue["Issue from the template:<br/>a Markdown request"] --> check["Issue workflow: companion action<br/>one comment with report and preview"]
  end
  subgraph catalog["2 · Catalog repo"]
    direction LR
    pr["Pull request adds<br/>schedules/name.canned-schedule.md"] --> ci["CI: schedule:render<br/>.json · .svg · .txt"]
  end
  subgraph publish["3 · Publish a version"]
    direction LR
    release["Git tag + release assets"]
    artifactory["Artifactory<br/>generic repository"]
  end
  subgraph consume["4 · Consume a pinned version"]
    direction LR
    capacity[/"capacity.json<br/>state id → capacity"/] --> infra["Terraform or AWS CLI<br/>ASG and ECS scheduled actions"]
    docs["Docs<br/>Sphinx kit, SVG previews"]
  end
  check -->|approved| pr
  ci --> release & artifactory
  release & artifactory --> infra
  release --> docs
```

1. **Propose:** someone opens an issue from the template, or proposes the schedule from a team's playground. The
   issue workflow checks the proposal and keeps one comment with the report and, for a valid schedule, the preview.
2. **Catalog:** once approved, the proposal becomes `schedules/<name>.canned-schedule.md` in a catalog repo. CI and
   the pre-commit hook check it; `schedule:render` writes the outputs. [`examples/consumer`](https://github.com/iilei/canned-schedules/tree/master/examples/consumer) has
   both steps ready to copy.
3. **Publish:** your pipeline's part. The outputs are plain files, so a git tag with release assets, an Artifactory
   generic repository, or both work. Publish the JSON at least; diffing its `base` and `week` against the previous
   version tells consumers whether the schedule itself changed.
4. **Consume:** infrastructure code pins a catalog version, reads the schedule's JSON and maps its state ids to
   capacities that stay with the infrastructure (the AWS recipes are under [Output](output.md)). Docs
   use the SVG previews or the [Sphinx kit](sphinx.md).

:::{toctree}
:caption: Write and check
:hidden:

playground
syntax
usage
previews
error-reports
examples
:::

:::{toctree}
:caption: Use the output
:hidden:

output
aws
configuration
catalog
sphinx
rendering
library
:::

:::{toctree}
:caption: Run a catalog
:hidden:

self-hosted-catalog
playground-links
stories/shift-swap/README
stories/classroom-capacity/README
:::

:::{toctree}
:caption: Project
:hidden:

stability
development
:::
