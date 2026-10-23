# Canned Schedules

## Example

Given Input (located at `commonmark/lenient-expressions.md`):

~~~markdown
---
doctype: canned-schedule
meta:
    not_before: 2026-12-24
    tiers:
    Tier-A:
        ref: "#/canned-tiers/tier-a"
    Tier-B:
        ref: "#/canned-tiers/tier-b"
    Tier-C:
        ref: "#/canned-tiers/tier-c"
---

# Schedule-Name

> [!TIP]
> Definition order matters: later definitions win when transitions conflict.

> [!Base]
> `Tier-A`

> [!Tier-A]
> `00:00 FRI`

> [!Tier-B]
> `01:00 SAT`

> [!Tier-A]
> `05:00 SUN`

> [!Tier-C]
> `08:00 MON-FRI`

> [!Tier-A]
> `18:00 MON-FRI`

~~~

### 1st Validation and Extraction

~~~shell
mise run md:extract commonmark/lenient-expressions.md
~~~

Output:

```json
{
  "frontmatter": {
    "doctype": "canned-schedule",
    "meta": {
      "not_before": "2026-12-24",
      "tiers": null,
      "Tier-A": {
        "ref": "#/canned-tiers/tier-a"
      },
      "Tier-B": {
        "ref": "#/canned-tiers/tier-b"
      },
      "Tier-C": {
        "ref": "#/canned-tiers/tier-c"
      }
    }
  },
  "base": "Tier-A",
  "transitions": [
    {
      "tier": "Tier-A",
      "expression": "00:00 FRI"
    },
    {
      "tier": "Tier-B",
      "expression": "01:00 SAT"
    },
    {
      "tier": "Tier-A",
      "expression": "05:00 SUN"
    },
    {
      "tier": "Tier-C",
      "expression": "08:00 MON-FRI"
    },
    {
      "tier": "Tier-A",
      "expression": "18:00 MON-FRI"
    }
  ]
}

```

### Todo: CUE

***

# Schedule expression syntax

The schedule expression regex recognizes the supported input formats. It performs syntax recognition only; semantic validation belongs to CUE after normalization.

## Supported formats

- **Friendly form:** `HH:MM DAY-SPEC`
  - Example: `00:00 FRI`, `08:00 MON-FRI`, `09:30 MON,WED,FRI`
- **Five-field cron form:** `MIN HOUR * * DAY-SPEC`
  - Example: `0 0 * * FRI`
- **Quartz-style six-field form:** `MIN HOUR ? * DAY-SPEC *`
  - Example: `0 0 ? * FRI *`

`DAY-SPEC` consists of weekday names (`MON`, `TUE`, `WED`, `THU`, `FRI`, `SAT`, `SUN`), inclusive ranges (`MON-FRI`), or comma-separated combinations of days and ranges (`MON-WED,FRI`).

Weekday selection is mandatory. `MON-SUN` explicitly represents every day.

## Tested Examples

~~~
0 0 ? * MON-WED,FRI *
00:00 FRI
08:00 MON-FRI
09:30 MON,WED,FRI
00:00 MON-WED,FRI
0 0 * * FRI
0 0 * * MON-FRI
0 0 ? * FRI *
0 0 ? * MON-WED,FRI *
~~~

## Intentionally unsupported

- Step expressions, such as `2/3 2 ? * FRI *`
- Wildcards or ranges in the minute and hour fields
- Numeric weekday selectors
- Advanced cron operators such as `L`, `W`, and `#`
- Other cron extensions not explicitly listed above

These forms are excluded because they can introduce recurring or otherwise more complex transition patterns. Supporting them would require explicit semantics in the schedule model and evaluator, not just a more permissive regex.

## Validation responsibilities

- **Regex / jq:** Recognize the supported input shapes and extract the tier and expression.
- **Normalization:** Parse recognized expressions into structured fields such as minute, hour, and weekdays.
- **CUE:** Validate field ranges, weekday order, duplicates, tier references, and schedule-level invariants.

Do not treat regex acceptance as proof that an expression is semantically valid. Extend the accepted grammar deliberately, with corresponding normalization and validation tests.
