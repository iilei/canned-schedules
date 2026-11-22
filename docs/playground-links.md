# Links into your playground

A playground opens what its address says: a schedule, the flags to check it with, the options, and whether the reader
may change any of that. A team hosting a playground can hand out links that open exactly the right setup, for a
catalog's schedule, a project's rules or a teaching example, without anyone setting it up by hand.

## Two kinds of playground

| Setup                                                                                                                        | Where the flags come from                               | Proposal buttons        |
| ---------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- | ----------------------- |
| **One catalog's playground**, with a `site.json` that sets `args` ([Your own playground](playground.md#your-own-playground)) | `site.json`; they always apply and nobody can drop them | yes, to that catalog    |
| **A shared playground**, without `site.json` or with one that sets no `args` (the public page is one)                        | the link (`?arg=`, `?sample-interval=`, `?states=`)     | only from a `site.json` |

On a catalog's playground, a link adds the schedule, the lock and the look; repeating the site's flags in `?arg=` is
reported as a conflict (`invalid-flag`), since the site's flags must stay the ones that decide. On a shared playground,
each project brings its own flags in the link, so one hosted page serves every project. It can't send a proposal
anywhere: targets come only from `site.json`, never from an address, so a crafted link can't redirect a schedule. Its
readers copy the checked schedule into their own forge.

## The parameters

| Parameter                                | Effect                                                                                                                                                                               |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `#doc=<base64url>`                       | The schedule to open: its UTF-8 text in base64url without padding. A fragment, so it never reaches the server.                                                                       |
| `?lock=true`                             | Locks everything except the Markdown: the preset, the options, the States panel and dropping the link's flags. The reader edits the schedule only.                                   |
| `?arg=`                                  | One argument each, repeated in order: `--require-base`, `--base`, `--error-unless-alert-pattern`, `--set-config-at` and `--set-config-value`, values as the next `arg` or after `=`. |
| `?sample-interval=`                      | The sample interval in minutes (a divisor of 60). The page has a control for it, so it isn't an `?arg=`.                                                                             |
| `?states=`                               | The states, repeated in order: `<id>` or `<id>:<display name>`.                                                                                                                      |
| `?preset=`                               | One of the page's built-in examples (`office-hours`, `support-desk`, `on-call-roster`, …), with its own flags.                                                                       |
| `?downsample=`, `?symbols=`, `?palette=` | How the previews look: bucket minutes, `ascii`, `braille` or `styles`, and a colour palette (`okabe-ito`, `classic`, …).                                                             |
| `?theme=`, `?view=`, `?tab=`             | `light`, `dark` or `system`; `sphinx` or `github`; `rendered`, `diagnostics`, `json` or `ascii`. They stay free under the lock.                                                      |

**Copy link** on the page writes every parameter of the current state, so the easiest way to a link is often to set the
page up and copy it. A flag the page can't take (output files, `--svg-theme`, unknown flags) is reported, and then the
link's flags are left out entirely; the page's own still apply.

## Recipes

**Edit a catalog's schedule** on the catalog's own playground: the schedule as it stands, locked, so the reader only
changes the text and then uses **Propose change**:

```text
https://schedules.example.com/?lock=true#doc=IyBPZmZpY2UgSG91cnMK…
```

**A project's rules on a shared playground**: the flags of the project's `canned-schedules.args`, locked, with or
without a schedule to start from:

```text
https://schedules.example.com/?lock=true&arg=--error-unless-alert-pattern&arg=%28%3Fi%29NOTE%7CTIP&sample-interval=15
```

**Teach with an example**: a preset and the view to look at, unlocked so the reader can explore:

```text
https://schedules.example.com/?preset=on-call-roster&tab=ascii&view=github
```

## A script for the links

`playground-link` writes a locked link from a project's `canned-schedules.args` (the file the bots and CI read, so the
link checks exactly as CI does) and, optionally, a schedule file. It moves the flags the page has a control for to
their own parameters. It needs `jq`.

```sh
#!/bin/sh
# playground-link BASE_URL [ARGS_FILE] [SCHEDULE_FILE]: prints a locked link to a playground that checks the schedule
# with a project's flags. ARGS_FILE is a canned-schedules.args (one argument per line; blank lines and # comments
# skipped); leave it out (or pass "") for a playground whose site.json already sets the flags.
set -eu
base=$1 args=${2:-} doc=${3:-}
q=lock=true
add() { q="$q&$1=$(printf '%s' "$2" | jq -sRr @uri)"; }
if [ -n "$args" ]; then
  set --
  while IFS= read -r line || [ -n "$line" ]; do
    line=${line%"$(printf '\r')"}
    case $line in '' | '#'*) continue ;; esac
    set -- "$@" "$line"
  done < "$args"
  while [ $# -gt 0 ]; do
    case $1 in
      # Flags the page has a control for travel as that control's parameter.
      --sample-interval=*) add sample-interval "${1#*=}" ;;
      --sample-interval) add sample-interval "$2"; shift ;;
      --ascii-symbols=*) add symbols "${1#*=}" ;;
      --ascii-symbols) add symbols "$2"; shift ;;
      --preview-downsample=*) add downsample "${1#*=}" ;;
      --no-preview-downsample) add downsample 1 ;;
      *) add arg "$1" ;;
    esac
    shift
  done
fi
url="${base%/}/?$q"
if [ -n "$doc" ]; then
  url="$url#doc=$(base64 < "$doc" | tr -d '\n' | tr '+/' '-_' | tr -d '=')"
fi
printf '%s\n' "$url"
```

```sh
# A project's rules and a schedule, on a shared playground:
playground-link https://schedules.example.com canned-schedules.args schedules/night-shift.canned-schedule.md
# A catalog's playground already has the flags: only the schedule and the lock.
playground-link https://catalog.pages.example.com "" schedules/office-hours.canned-schedule.md
```

Checked against the public page: a link from an args file with an alert pattern, `--sample-interval 15` and
`--require-base true` opens locked, with the sample interval set and "This link sets 2 flags of its own".

## Things to know

- **The lock isn't enforcement.** Anyone can remove `lock=true` from the address. It keeps the page honest about who
  owns the flags; what a merge request is held to is the review, which checks with the target branch's
  `canned-schedules.args`. A catalog's playground applies its `site.json` flags whatever the link says.
- **Length:** the schedule makes the link grow. Browsers take long addresses, but chat tools and forges may cut them;
  a proposal from the page copies the schedule to the clipboard above `max-length` (8000 by default, about GitLab's
  limit).
- **The issue preview's playground link** (the companion's `preview render --fiddle-link-base-uri`) is built the same
  way, with the run's flags as `?arg=`.
