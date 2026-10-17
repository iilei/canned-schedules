
# Schedule Tiers

See below for the proposed Weekdays and hours for a proposed *canned schedule*.

Weekday patterns and time ranges are freely editable. Ranges are half-open ([start, end)): the start is included, the end is
not — so `00:00-06:00` means `[00:00, 06:00)`.

A time slot may span midnight. The portion after midnight is treated as belonging to the following day, even if that day is not
included in the configuration's weekday set.

> [!NOTE]
>
> **Chose from Tiers:**
>
> * [x] Offline
> * [x] Minimum
> * [x] Regular
> * [x] Maximum
>
> **How to edit:**
>
> Follow the example below, treat line breaks and Lines starting with `#` (hashmark character)
> as mandated sections. These Tier-determining lines are displayed as 2nd order headlines in preview.

## :card_index_dividers: Offline

Capacity scaled to zero.

**ALL-WEEK**

* [ ] 24h

## :card_index_dividers: Minimum

Capacity scaled down, in service.

**ALL-WEEK**

* [x] 24h

## :card_index_dividers: Regular

Capacity suitable for regular load.

**MON-FRI**

* [x] 07:00-12:00
* [x] 14:00-18:00

**SAT,SUN**

* [x] 06:00-14:00
* [x] 15:00-18:30

## :card_index_dividers: Maximum

Capacity suitable for extraordinary high load.

**ALL-WEEK**

* [ ] 24h
