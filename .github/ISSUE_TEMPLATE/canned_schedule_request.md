---
name: Schedule Request
about: Request a Schedule
title: "[schedule-contrib]: "
labels: ["Schedule", "triage"]
assignees: ""
---

# Schedule Tiers

See below for the proposed Weekdays and hours for a proposed *canned schedule*.

> **How to edit:**
>
> Follow the example below, treat line breaks and Lines starting with `#` (hashmark character)
> as mandated sections. These Tier-determining lines are displayed as 2nd order headlines in preview.

## :card_index_dividers: Offline

Capacity scaled to zero.

**ALL-WEEK**

+ n/a

## :card_index_dividers: Minimum

Capacity scaled down, in service.

**ALL-WEEK**

+ full-day

## :card_index_dividers: Regular

Capacity suitable for regular load.

**MON-FRI**

+ 06:00-22:00

**SAT,SUN**

+ 10:00-24:00

## :card_index_dividers: Maximum

Capacity suitable for extraordinary high load.

**ALL-WEEK**

+ full-day

***

Weekday patterns and time ranges are freely editable. Ranges are half-open ([start, end)): the start is included, the end is
not — so `00:00-06:00` means `[00:00, 06:00)`.

A time slot may span midnight. The portion after midnight is treated as belonging to the following day, even if that day is not
included in the configuration's weekday set.

> [!NOTE]
>
> **Chose from Tiers:**
>
> + Offline
> + Minimum
> + Regular
> + Maximum
>
