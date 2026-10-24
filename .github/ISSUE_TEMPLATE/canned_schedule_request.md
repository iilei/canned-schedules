---
name: Schedule Request
about: Propose a canned capacity schedule
title: "[schedule-contrib]: "
labels: ["Schedule", "triage"]
assignees: ""
---

# Office Hours

<!--
How to edit this request

- Replace the name above (the H1). Letters, digits, spaces and hyphens; up to 80 characters.
- Keep each block as its own quote: a line starting with `> [!SCHEDULE:…]`, followed by its list.
- Change the tiers, times and weekdays below. Add or remove TIER blocks as needed.
- Everything outside these blocks is free text for reviewers: explain why you need this schedule.

The full syntax reference is at the end of this request.
-->

> [!SCHEDULE:META] Weekdays with a late Friday
> - x-custom-owner: `team-xyz`

> [!SCHEDULE:BASE] Minimum

> [!SCHEDULE:TIER] Medium
> - `06:00` to `22:00` MON-FRI
> - `10:00` to `18:00` SAT

> [!SCHEDULE:TIER] Maximum
> - `16:00` to `00:00` FRI

## Why this schedule

<!-- Describe the workload and why these hours fit it. -->

## Syntax reference

> [!NOTE]
> **Blocks**
>
> - `[!SCHEDULE:META]` (optional): text after the marker is a short hint. List items are free-form entries for
>   your own bookkeeping, written like `` x-custom-owner: `team-xyz` ``.
> - `[!SCHEDULE:BASE] <tier>` (optional): the tier wherever no range applies.
> - `[!SCHEDULE:TIER] <tier>`: one block per tier, each with a list of ranges.
>
> **Tiers:** `Off`, `Minimum`, `Medium`, `Maximum`.
>
> **Ranges:** `` `start` to `end` WEEKDAYS ``
>
> - Times are `H:MM` or `HH:MM`, e.g. `6:15` or `21:45`.
> - The end is exclusive: `06:00` to `22:00` covers 06:00 up to, but not including, 22:00.
> - A range that runs until midnight ends at `00:00`. There is no `24:00`.
> - A range may cross midnight: `22:00` to `02:00` FRI runs from Friday 22:00 into Saturday 02:00.
> - Equal start and end means a full 24 hours: `08:00` to `08:00` MON runs until Tuesday 08:00.
>
> **Weekdays:** `MON TUE WED THU FRI SAT SUN`, uppercase. Ranges like `MON-FRI` (also across the weekend,
> `FRI-MON`) and lists like `SAT,SUN`.
>
> **Overlaps:** later ranges win where they overlap earlier ones.
