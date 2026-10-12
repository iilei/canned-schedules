**P1 — Rule-based input with CUE validation**

1. **Define the rule format and its exact meaning.** A proposal has a `default` tier plus rules shaped as `{days, from, to, tier}`.
   - `from` is inclusive and `to` is exclusive.
   - Times must sit on the 30-minute grid, and `24:00` is allowed as an end time.
   - An overnight range keeps going into the next day. `days` means the day the range starts, so Sunday wraps into Monday.
   - Rules may not overlap, and no rule takes priority over another.
2. **Turn the rules into slots in CUE.** CUE converts them into the existing 7 × 48 weekly schedule, so rendering and `aws_cron` don't change.
3. **Validate the inputs in CUE.** Reject times off the 30-minute grid, `from == to`, and unknown days or tiers, with readable messages written with `error()`.
4. **Detect overlaps in CUE.** Count how many rules cover each slot, including across midnight and from Sunday into Monday. The error message names the day, the time, and the conflicting rules.
5. **Add `schedule_rules` to `proposal.sh`.** It becomes a third input form, with only one form allowed per proposal. Update the `jq` list of allowed fields and keep the existing patch generation.
6. **Add tests.** Cover a weekday rule, an overnight rule, the Sunday-to-Monday wrap, overlaps, off-grid times, unknown values, and `from == to`, and check the error text.

**P2 — Make previews easier to understand**
7. **Add plain-language summaries.** Merge slots into intervals for each day or group of days and show them above the chart.
8. **Clarify labels and the cron section.** Say that slots are 30 minutes and ranges are end-exclusive. Move the cron list into an "Automation details" section that explains what a transition means.
9. **Explain each tier in the legend.** Show its min, desired, and max values next to the symbol.
10. **Make the SVG readable without hovering.** Put interval labels on the chart itself and keep hover for extra detail.

**P3 — Rollout and polish**
11. **Update the README, demo proposal, and `/contribute-schedule` skill** so they use the rule format.
12. **Optional: report overlaps as ranges.** For example, show `MON 09:00–10:00` as one conflict instead of two half-hour conflicts.

**Ideas — being explored**
13. **Guided tour as a `mise run tour` task.** An interactive walkthrough (`raw = true`) that explains how to read a preset, then optionally asks questions to draft a proposal and runs `preview-proposal`. Include a non-interactive mode so `mise run check` can exercise it. The drafting part should emit the rule format from item 1.
14. **Paint-the-grid editor built on the existing SVG.** A static HTML page that embeds a preset SVG inline (`turned_off` as a blank canvas, or any preset to edit) and explains how to draw. Pick a tier, then click or drag over slots; JavaScript swaps each rect's `schedule-tier-N` class, so the existing light and dark colors still apply.
   - Add `data-day` and `data-slot` attributes to the slot rects in `schedule.svg.tpl`. This doesn't change how the SVG looks.
   - While editing, hide the cron list and hover text or mark them as stale, because both come from the original CUE export.
   - A download button writes a proposal `.md` in the existing `schedule_visual` format, and `preview-proposal` validates it as usual.
   - Generate the page as one self-contained HTML file with the SVG and script inlined, so it opens directly from disk (`file://`).
   - Saving uses a browser download (Blob + `<a download>`) and a "copy to clipboard" button for pasting into an issue. No API is needed.
   - It needs no framework, server, or Docker. For a terminal version, see item 16.
16. **Terminal editor with Bubble Tea and the CUE Go API (`mise run edit-schedule`).** Paint the 7 × 48 grid in the terminal, keyboard-first (arrow keys, `0`–`3` to pick a tier, select-and-fill), with mouse dragging where the terminal supports it.
   - Evaluate the repo's CUE files in-process with `cuelang.org/go`, loaded from the repo at runtime so the catalog stays the single source of truth. After each edit, read back the validation result and `aws_cron`, so cron transitions update live without Docker or a server.
   - Use the structured CUE errors (path and position) to point at the affected day and time in the grid, which also helps with friendly errors (items 3, 4, 12).
   - Save the proposal `.md` into `demo/` and run the existing preview and patch flow. Like item 15, it doesn't change `schedules.cue` directly.
   - Could also host the guided tour (item 13).
   - Cost: adds Go to `mise.toml` and a small Go module to maintain. Check Unicode block and color rendering in Windows Terminal, macOS Terminal, and common Linux terminals.

**Shelved — revisit after the items above**
15. **Optional Docker mode for the editor (`mise run editor`).** Same page as item 14, plus a small local server in a container with the pinned CUE, gomplate, jq, and yq versions and the repo bind-mounted.
   - `POST /preview`: after each paint stroke (debounced), run the existing `proposal.sh` and render pipeline and return the fresh SVG with cron transitions, plus summary text once item 7 exists. CUE stays the only validator, and its errors are shown in the page.
   - `POST /save`: write the proposal `.md` and its generated previews and patch into `demo/`, exactly as `preview-proposal` does. It doesn't change `schedules.cue`; applying the patch stays a reviewed step.
   - Bind to `127.0.0.1` only, allow writes only under `demo/`, check the schedule name, and run as the host user's UID so created files aren't owned by root.
   - The static file from item 14 stays the fallback for anyone without Docker.

The P2 work doesn't depend on P1, so it can run in parallel.
