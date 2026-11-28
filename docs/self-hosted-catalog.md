# How to host your own schedule catalog

A **schedule catalog** is a repository that holds a team's approved weekly schedules: one `*.canned-schedule.md` file
per schedule in the catalog's folder (`schedules/`), checked on every change, with a playground where anyone can draft a
schedule and propose it. This guide sets
one up on **GitLab**, step by step, with released building blocks only:

- [canned-schedules](https://github.com/iilei/canned-schedules) checks and renders schedules. Every release attaches
  the playground as `canned-schedules-site_<version>.zip`.
- [canned-schedules-companion](https://github.com/iilei/canned-schedules-companion) brings four GitLab CI/CD
  components: the **issue preview**, the **playground on GitLab Pages**, the **merge request review** and, optionally,
  **publishing** the merged schedules.

At the end, the flow looks like this:

1. Someone opens the playground on your Pages site, writes a schedule and sees its week, checked with your
   catalog's flags.
2. **Propose new schedule** opens GitLab's new-file page with the file named after the schedule
   (`Night Shift` → `schedules/night-shift.canned-schedule.md`) and the schedule filled in. They commit it to a new
   branch and open a merge request.
3. The review job checks the change, posts one note with the report and the week, sets the `Schedule` label and
   renames the merge request from "Add new file" to "Schedule: Night Shift".
4. You merge. The next pipeline republishes the playground, which now knows the schedule and offers **Propose change**
   for it, and, if you set it up, publishes the schedule's outputs to your artifact store.

People who start on GitLab instead open an issue from a template, and the issue preview answers there.

For a change to an existing schedule, follow [two people swapping an on-call shift](stories/shift-swap/README.md),
with a screenshot of each step.

## What you need

- A GitLab project for the catalog, with **GitLab Pages** enabled on the instance and the project.
- A runner that picks up the project's jobs and can download from github.com (the components fetch signed releases
  from there and verify them with GPG).
- Maintainer rights in the project, for tokens, variables, webhooks and branch protection.

## 1. Lay out the repository

```text
schedules/                       the catalog's folder: one file per schedule
  office-hours.canned-schedule.md
canned-schedules.args            the flags every check uses, one argument per line
.gitlab/issue_templates/         optional: templates for proposals by issue
.gitlab-ci.yml
```

`canned-schedules.args` is what keeps every check consistent: the playground, the issue preview and the merge request
review all read it. A good start rejects alert types that aren't GitHub's or GitLab's, so a misspelled
`[!SCHEDUEL-STATE]` doesn't pass as prose:

```text
# One argument per line; blank lines and lines starting with # are skipped.
--error-unless-alert-pattern
(?i)NOTE|TIP|IMPORTANT|WARNING|CAUTION
```

Name every file after its schedule's title (`# Office Hours` → `office-hours.canned-schedule.md`); the review checks
that. The core's [`examples/consumer`](https://github.com/iilei/canned-schedules/tree/master/examples/consumer) has the issue templates and local tasks to copy.

## 2. Protect the default branch

GitLab's web editor commits **straight to the default branch** unless the author picks "Commit to a new branch", and
the playground's proposals end in that editor. Under Settings > Repository > Protected branches, allow pushes to the
default branch for Maintainers only (or no one) and merges for whoever approves schedules. Proposals then always become
merge requests, and the review sees every one of them.

## 3. Create the bot's token

Under Settings > Access tokens, create a **project access token** with the `api` scope and the **Developer** role. The
role can't be changed later (GitLab refuses to change a project bot's role), and labelling and renaming merge requests
needs Developer. Store the token as the CI/CD variable `CANNED_SCHEDULES_TOKEN`, masked. The issue preview and
publishing need only Planner; the companion's
[Tokens and permissions](https://iilei.github.io/canned-schedules-companion/permissions.html) lists every step's role.

Before it expires, **rotate** the token and store the new value: rotation keeps the bot user, so it goes on updating its
own notes. A new token is a new bot, which can't edit the old bot's notes.

## 4. Include the components

```yaml
include:
  # Issues with a schedule get a preview note.
  - remote: https://raw.githubusercontent.com/iilei/canned-schedules-companion/refs/tags/v0.3.1/templates/issue-preview.yml
    integrity: sha256-vxBQa9Sd3Yvp5HAeCpL8Igk57l6MKlFRyhBTCWLp7wo=
    inputs:
      args: |
        --error-unless-alert-pattern
        (?i)NOTE|TIP|IMPORTANT|WARNING|CAUTION
  # The default branch publishes the playground on GitLab Pages.
  - remote: https://raw.githubusercontent.com/iilei/canned-schedules-companion/refs/tags/v0.3.1/templates/playground.yml
    integrity: sha256-Ywvz35MA3qorhf/LZvM3JGmTUFbuCqFXuLi1J3NDCsk=
    inputs:
      name: Acme schedules
  # Merge requests that change schedules/ get reviewed.
  - remote: https://raw.githubusercontent.com/iilei/canned-schedules-companion/refs/tags/v0.3.1/templates/review.yml
    integrity: sha256-3CF1y1zQo117XM/wMfLIFP67pmNeQduvaHIktpFzQnI=
```

GitLab refuses an included file that doesn't match its `integrity`. The [companion's docs](https://iilei.github.io/canned-schedules-companion/gitlab.html) list every input; take the
tag and hashes of its newest release from there. The issue preview takes its flags as an input, while the playground
and the review read `canned-schedules.args` from the repository. The review reads it **from the target branch**, so
a merge request can't loosen its own check.

The playground job is named `pages`, so the project can't have another `pages` job.

## 5. Wire the issue preview (optional)

GitLab can't start a pipeline from an issue event by itself:

1. Create a pipeline trigger token under Settings > CI/CD > Pipeline trigger tokens.
2. Add a webhook under Settings > Webhooks with **Issues events** (and confidential issues, if you use them), pointing
   at `https://<gitlab>/api/v4/projects/<project ID>/ref/<default branch>/trigger/pipeline?token=<trigger token>`.

An issue that contains a schedule now gets a note with the report and the week, and the `Schedule` label while the
schedule is valid. Issues that only mention the syntax in backticks are left alone.

## 6. Publish and try it

The first pipeline on the default branch publishes the playground. GitLab shows its address under Deploy > Pages:
usually a unique domain per project (`https://<project>-<hash>.<pages domain>/`); the older path form redirects there.

Open it and walk through the flow from the top: draft a schedule, **Propose new schedule**, commit to a new branch with
a merge request, watch the review, merge, reload the playground.

## 7. Publish what you merge (optional)

The catalog's files are the source; consumers such as a Terraform module want the rendered outputs, in one place,
with a record of which review they passed. The companion's **publish** component does that on every push to the
default branch that changes a schedule (from companion v0.3.5; the upload scripts below from v0.3.8):

```yaml
include:
  - remote: https://raw.githubusercontent.com/iilei/canned-schedules-companion/refs/tags/v0.3.8/templates/publish.yml
    integrity: sha256-5woxUGdXNDn9PfqMPiQK3YIQaN5aI6HpS1S4pHGUtKg=
    inputs:
      post-fetch-script: scripts/upload-schedules.sh
```

For each changed schedule it writes one folder, named by the `path-template` input (by default
`<slug>/<merge request>-<12 hex of the digest>`), with the file as merged, its JSON, SVG and text form, and a
provenance file: the merge request, who merged it and when, the review and the versions that rendered it.
`manifest.json` lists them all, with their metadata ready for Artifactory or S3. Then it runs your script with the
folder as its argument.

The companion ships the script: copy the [upload script](https://iilei.github.io/canned-schedules-companion/publishing.html)
for Artifactory (or S3) from the release you pin into your repository. It runs with your secrets, so it belongs under
your own review:

```sh
curl -fsSLo scripts/upload-schedules.sh \
  https://raw.githubusercontent.com/iilei/canned-schedules-companion/refs/tags/v0.3.8/publish/upload-artifactory.sh
```

It uploads every file with its three checksums (Artifactory verifies each upload) and the schedule's metadata as
properties, and it never overwrites: a file already there with the same content is skipped, so a rerun passes, and
one with other content fails. That suits a repository that keeps artifacts immutable. Set the CI/CD variables
`ARTIFACTORY_URL` (e.g. `https://acme.jfrog.io/artifactory`), `ARTIFACTORY_REPO` and `ARTIFACTORY_TOKEN` (masked;
or `ARTIFACTORY_USER` and `ARTIFACTORY_PASSWORD`), and optionally `ARTIFACTORY_PATH` for a folder in the repository.
Consumers find a schedule's newest publish by its properties (`schedule`, `merged-at`), for example with an AQL
query. The job's `image` input (Alpine by default) gets `curl` and `jq` from the script; for S3, pick an image with
the AWS CLI.

**Published is always what was merged.** Every schedule is rendered again from the file as merged, with the
`canned-schedules.args` of the merged commit. The review vouched for each file by its digest (a SHA-256 of its bytes,
written in the review note); when the merged file has another digest, because someone pushed after the review with
the pipeline skipped or a conflict was resolved in the merge, the schedule is still published, marked
`reviewed: false` with the reason, and a note on the merge request lists the file with both digests. A push straight
to the default branch, without a merge request, fails the job and publishes nothing: one more reason for step 2.

## Things to know

- **`site.json` is as public as the Pages site.** It holds no secrets, but it lists your schedules' names. For a
  private catalog, set the Pages visibility to project members (Settings > General > Visibility > Pages).
- **Links for your readers:** `?lock=true#doc=…` opens a schedule with the controls locked, for example to edit a
  catalog file; [Links into your playground](playground-links.md) has the parameters and a script that builds them.
- **Readers who aren't members** need a fork before GitLab opens the new-file and edit pages for them. Set the
  playground input `fork: true` to show them a link to the fork page.
- **Merge requests from forks** run their pipelines in the fork, without `CANNED_SCHEDULES_TOKEN`. Let the project run
  fork pipelines in the parent, or review those by hand.
- **Only real changes start a check:** an edit that changes nothing in a description, or a label change, doesn't
  trigger the issue preview, and a run for an older commit or text writes nothing.
- **Updates:** pin a companion release and move up deliberately; each release names the core version it carries, and
  the playground follows it unless you set `site-version`.

## Try it on your machine first

A GitLab in Docker works for a trial, with a few adjustments:

- **Pages:** in `GITLAB_OMNIBUS_CONFIG`, set `pages_external_url 'http://pages.localhost:8091'`,
  `gitlab_pages['enable'] = true` and `pages_nginx['listen_port'] = 8091`, and publish port `8091:8091`. Not 8090:
  the Pages daemon uses it inside the container. Browsers resolve `*.localhost` to your machine.
- **http on your machine:** proposal targets must be https, except for `localhost`, `*.localhost`, `127.0.0.1` and
  `[::1]`. Set the playground input `server-url: http://localhost:8080` (your GitLab's address in the browser).
- **GitLab calls itself by its container name:** the webhook's URL starts with `http://gitlab/…` (`http://localhost/…`
  fails inside the container), and the components need `api-url: http://gitlab/api/v4` in jobs.
- **Local webhooks** must be allowed: Admin > Settings > Network > Outbound requests.

## Without the companion

The playground is a static page: download `canned-schedules-site_<version>.zip` from a
[canned-schedules release](https://github.com/iilei/canned-schedules/releases), check it against the signed
`checksums.txt`, unzip it and add a `site.json` (see "Your own playground" in the [README](playground.md#your-own-playground)).
Any static host serves it. The companion's `playground build` writes that `site.json` for you, and its README shows
the command.

## On GitHub

The same catalog works on GitHub with the companion's GitHub Actions (from v0.3.3; `publish` from v0.3.5): the issue preview at the
repository root, `playground` for GitHub Pages and `review` for pull requests. The
[companion's docs](https://iilei.github.io/canned-schedules-companion/github.html#github-pages-playground) have the workflows to
copy. What differs from GitLab:

- **Pages:** set Settings → Pages → Source to "GitHub Actions". The `playground` action builds and uploads the site,
  and a second job deploys it with `actions/deploy-pages`, since only a job can name the `github-pages` environment.
- **No token to create:** the workflow token comments, labels and renames, with `contents: read` and
  `pull-requests: write` on the review job (a pull request's comments and labels need `pull-requests: write`;
  `issues: write` alone is refused). Comments come from `github-actions[bot]`.
- **Propose change** copies the schedule and opens GitHub's edit page; **Propose new schedule** opens the new-file
  page filled in, and the review renames GitHub's default title ("Create night-shift.canned-schedule.md").
- **Pull requests from forks** get a read-only token: their review goes to the job summary, not into a comment.
- **Protect the default branch** with a ruleset that requires pull requests, so changes from the playground arrive as
  pull requests rather than commits.
- **Publishing** is the `publish` action, on `push` to the default branch with `contents: read` and
  `pull-requests: write` (for the note); the post-fetch script's secrets go in the `env` of the step that uses it.

The demo repository [`canned-schedules-demo`](https://github.com/iilei/canned-schedules-demo) runs this setup; its
playground is at <https://iilei.github.io/canned-schedules-demo/>. Follow
[an issue becoming a capacity schedule](stories/classroom-capacity/README.md) there, with a screenshot of each step: the
issue preview, the approval label and the pull request the bot opens.
