# Sphinx

The preflight page's Sphinx view is not a mock-up: the same look is available for real Sphinx docs built with the
[Furo](https://pradyunsg.me/furo/) theme. `--sphinx-kit-output-file kit.zip` (or **Download Sphinx kit** on the page)
writes a zip with:

- a local Sphinx extension (`_ext/canned_schedules`) that adds the stylesheet and fonts and enables MyST's
  `colon_fence`;
- the stylesheet, generated from the same state styles as the SVG and the page. It styles only schedule admonitions,
  through Furo's own admonition variables, and follows Furo's light, dark and auto modes;
- Ubuntu and Ubuntu Mono, used only inside schedule admonitions;
- the schedule as MyST, and a README with the two `conf.py` lines:

  ```python
  sys.path.append(os.path.abspath("_ext"))
  extensions += ["canned_schedules"]
  ```

`--myst-output-file schedule.myst.md` writes just the MyST for a schedule: the name, the summary, and the META, BASE and STATE
blocks as `:::{admonition}` blocks with the classes the stylesheet expects, items written as in the schedule. Free text
stays out. Colours follow `--set-config-*`; regenerate the kit after changing the state styles. On the page,
**Download MyST** gives the same file for the document in the editor.

## From schedule to docs page

Treat the MyST as build output: people edit and review the GitHub-flavoured schedule, and the docs build regenerates
the MyST from it. Nobody edits the MyST by hand. Validation then stays in one place, because the tool writes no MyST
for an invalid schedule and exits with 1.

In CI, run a step like this before `sphinx-build`:

```sh
for src in schedules/*.canned-schedule.md; do
  name=$(basename "$src" .canned-schedule.md)
  canned-schedules --myst-output-file "docs/schedules/$name.md" "$src" >/dev/null
done
```

- **Invalid schedules fail the build.** The error report goes to stderr; use `--error-output-file` to keep it.
- **Generated files:** either add `docs/schedules/` to `.gitignore`, or commit the files and let CI regenerate them
  and run `git diff --exit-code docs/schedules`, so they can't drift from the schedules.
- **Each generated file is a page:** list them in a toctree, for example with `:glob:` and `schedules/*`. Put prose
  about a schedule on a hand-written page that links to it.
