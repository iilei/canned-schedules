# Try it in the browser

The **preflight page** runs the same checker, compiled to WebAssembly, entirely in your browser: start from a preset
(from a bare "Medium all week" to the full issue template), edit it in a highlighting editor that marks errors and
warnings on their lines, and see the week as SVG, the issue as GitHub renders it, a Sphinx handbook view with admonitions coloured from the state
styles (downloadable as a [Sphinx kit](sphinx.md)), diagnostics that jump to the offending line, the JSON and the ASCII preview
(downloadable as the [text form](previews.md#text-form)). **States** lets you check against your own states (on-call
rotations, support levels) instead of the built-in capacity ones, and **Equivalent CLI flags** shows the flags that
give the CLI the same result, as a shell command or as lines for an args file. **Colours** offers palettes inspired
by Grafana's (sequential for ordered states, categorical for rotations, and the colour-blind safe Okabe–Ito). Light and dark themes included;
nothing is uploaded. **Copy link** gives a URL with the document and all settings to share; the document travels in
the URL fragment, which browsers never send to the server. The page's SVG, shown and downloaded, always embeds the
schedule in canonical form (like `--svg-embed-source`) and lists the annotations (like `--svg-annotations`), so a downloaded
picture stays traceable to its schedule. The line under the cursor shows what it produces (and the editor marks the lines that produce it): a range, its STATE
block or the BASE block sprinkles black-and-white dots on the week where it paints (sparser where a later range
paints over it), the H1 and META items get a frame in the SVG, and the JSON tab marks the lines each one produces (for the
META marker: summary and the whole `annotations` object; for ranges: the runs and transitions where its state
shows). A status bar under the editor explains it with the
[paint trace](previews.md#paint-trace), including ranges that have no effect. It is published from `master` at **<https://iilei.github.io/canned-schedules/>**; locally:

```sh
# Builds dist/site and serves it on http://127.0.0.1:8080 (--port to change).
mise run web:serve
```

## Your own playground

**[How to host your own schedule catalog](self-hosted-catalog.md)** walks through the whole setup on GitLab:
catalog, playground on Pages, issue preview and merge request review.

A team can host the page itself and set it up for its catalog with a `site.json` next to `index.html`
([schema](https://github.com/iilei/canned-schedules/blob/master/schema/site.schema.json)). Every release attaches the page as `canned-schedules-site_<version>.zip`
(`index.html` at the root, listed in the signed `checksums.txt`): unzip it into your Pages folder, add `site.json`
and publish; `mise run web:build` builds the same from a checkout. With it, the page checks every schedule with the catalog's flags, and a valid
schedule goes straight to the forge: **Propose new schedule** opens the forge's new-file page with the file named after
the schedule's title (`Night Shift` → `schedules/night-shift.canned-schedule.md`), **Propose change** does the same for
a schedule the catalog already has, and **Open as issue** opens a new issue. Merging stays with the repository's rules.

```json
{
  "name": "Acme schedules",
  "args": ["--error-unless-alert-pattern", "(?i)NOTE|TIP|IMPORTANT|WARNING|CAUTION"],
  "catalog": {"dir": "schedules", "names": ["office-hours", "on-call"]},
  "propose": {
    "create": "https://gitlab.example.com/acme/catalog/-/new/main?file_name={path}&content={content}",
    "update": "https://gitlab.example.com/acme/catalog/-/edit/main/{path}?content={content}",
    "issue": "https://gitlab.example.com/acme/catalog/-/issues/new?issue[title]={name}&issue[description]={content}",
    "fork": "https://gitlab.example.com/acme/catalog/-/forks/new"
  }
}
```

- `args` are the flags of your `canned-schedules.args`: `--require-base`, `--base`, `--error-unless-alert-pattern`,
  `--sample-interval` and `--set-config-at` / `--set-config-value`. They decide: the page applies them last, so its
  controls, a preset or a link can't loosen them. A link (`?arg=`) or preset that sets one of them is reported as an
  error, the sample-interval control shows the site's value and is locked, and so is the States panel when the site
  sets the states.
- `catalog.names` lists the schedules the catalog has (file names without `.canned-schedule.md`), so the page knows
  whether a schedule is new or a change. Regenerate the file when the catalog changes; a stale list only means the forge
  reports that the file exists.
- In the templates, `{path}`, `{content}` (the schedule verbatim, exactly as written in the editor, prose included) and
  `{name}` are filled in, URL-encoded. Only
  https URLs, except for a forge on your own machine (`localhost`, `*.localhost`, `127.0.0.1`, `[::1]`), which may
  use http, e.g. a GitLab in Docker for a demo. On GitHub, use `/new/<branch>?filename={path}&value={content}` for `create`, and
  `{"url": "https://github.com/OWNER/REPO/edit/<branch>/{path}", "prefill": false}` for `update`: GitHub's edit page
  takes no content, so the page copies the schedule and opens the page. Above `max-length` (default 8000, GitLab's
  limit is about 8 KB) the schedule goes to the clipboard as well.
- `fork`: on GitLab, readers who aren't members of the project need a fork before the new-file and edit pages open;
  the page links there. Leave it out when every reader is a member.
- Protect the default branch: members with push rights could otherwise commit straight to it from those pages.

Only `site.json` sets proposal targets, never the page's address, so a link can't send a schedule anywhere else. A
`site.json` with errors is ignored as a whole, and the page says why. The public page has none.

**[Links into your playground](playground-links.md)** shows how to hand out links that open a schedule with a
project's flags, locked, on a catalog's playground or on one playground shared by many projects.

`site.json` is as public as the Pages site: it holds no secrets, but it does list the catalog's schedules. For a
private catalog, restrict who can see the Pages site (GitLab Pages can be limited to project members; on GitHub it
depends on the plan). Regenerate `site.json` on every deploy, so `catalog.names` stays current.
