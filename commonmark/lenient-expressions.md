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
