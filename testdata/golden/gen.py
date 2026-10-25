"""Generate the golden fixtures in this directory.

Usage (from the repo root):

    python3 testdata/golden/gen.py

The fixtures are the spec. This script is just a convenient way to write them: valid cases get their expected output
from a tiny reference painter that follows CLAUDE.md (half-open ranges, the start day owns the range, start == end
means 24 h, later ranges win), and diagnostic positions are found by searching for a marker substring in the input.

Rules:
- Never derive fixtures from the Go implementation. If Go and a fixture disagree, check the spec first.
- Add or change cases here, run the script, then review the diff in testdata/golden like any other change.
- The script deletes and rewrites valid/, warn/ and error/, so cases that only exist on disk are removed.
"""

import hashlib
import json
import os
import shutil
import sys

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.dirname(os.path.abspath(__file__))
for group in ("valid", "warn", "error"):
    shutil.rmtree(os.path.join(OUT, group), ignore_errors=True)
DAYS = ["MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"]
STATES = ["Off", "Minimum", "Medium", "Maximum"]
WEEK = 7 * 1440


def hm(m):
    return f"{m // 60:02d}:{m % 60:02d}"


def paint(base, ranges):
    """ranges: list of (start 'HH:MM', end 'HH:MM', [days], state) in source order."""
    week = [base] * WEEK
    for start, end, days, state in ranges:
        sh, sm = start.split(":")
        eh, em = end.split(":")
        s = int(sh) * 60 + int(sm)
        e = int(eh) * 60 + int(em)
        length = (e - s) % 1440 or 1440
        for d in days:
            off = DAYS.index(d) * 1440 + s
            for i in range(length):
                week[(off + i) % WEEK] = state
    out = {}
    for di, d in enumerate(DAYS):
        runs = []
        for m in range(1440):
            t = week[di * 1440 + m]
            if runs and runs[-1]["state"] == t:
                runs[-1]["end"] = m + 1
            else:
                runs.append({"start": m, "end": m + 1, "state": t})
        out[d] = [{"start": hm(r["start"]), "end": hm(r["end"] % 1440), "state": r["state"]} for r in runs]
    return out


def paint_minutes(base, ranges):
    week = [base] * WEEK
    for start, end, days, state in ranges:
        sh, sm = start.split(":")
        eh, em = end.split(":")
        s = int(sh) * 60 + int(sm)
        e = int(eh) * 60 + int(em)
        length = (e - s) % 1440 or 1440
        for d in days:
            off = DAYS.index(d) * 1440 + s
            for i in range(length):
                week[(off + i) % WEEK] = state
    return week


# Sunday-first order, as both cron dialects number days.
SUN_FIRST = ["SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"]


def compress(indexes, label):
    """Render sorted Sunday-first day indexes as ranges and lists, never wrapping around the week."""
    if len(indexes) == 7:
        return "*"
    parts, run = [], [indexes[0]]
    for i in indexes[1:]:
        if i == run[-1] + 1:
            run.append(i)
        else:
            parts.append(run)
            run = [i]
    parts.append(run)
    return ",".join(label(r[0]) if len(r) == 1 else f"{label(r[0])}-{label(r[-1])}" for r in parts)


def transitions(week):
    """Every minute where the state changes (cyclic), grouped by (time of day, state)."""
    groups = {}
    for m in range(WEEK):
        if week[m] == week[m - 1]:
            continue
        key = (m % 1440, week[m])
        groups.setdefault(key, []).append(m)
    out = []
    for (tod, state), minutes in sorted(groups.items(), key=lambda kv: (min(kv[1]), kv[0][1])):
        days = sorted({m // 1440 for m in minutes})
        sun_first = sorted(SUN_FIRST.index(DAYS[d]) for d in days)
        h, mi = divmod(tod, 60)
        entry = {"time": hm(tod), "days": [DAYS[d] for d in days], "state": state}
        # The short sha covers what the transition means, not the derived cron strings.
        sha = hashlib.sha256(json.dumps(entry, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()[:8]
        out.append({
            **entry,
            "cron": {
                "posix": f"{mi} {h} * * {compress(sun_first, str)}",
                "aws": f"{mi} {h} ? * {compress(sun_first, lambda i: SUN_FIRST[i])} *",
            },
            "sha": sha,
        })
    return out


def output(base, ranges, annotations=None, name="Schedule-Name", summary=None, warnings=()):
    out = {"schema-version": "1", "name": name}
    if summary is not None:
        out["summary"] = summary
    week = paint_minutes(base, ranges)
    week_runs = paint(base, ranges)
    # The schedule digest covers what a consumer acts on: the base state and the painted week.
    schedule = json.dumps({"base": base, "week": week_runs}, separators=(",", ":"), ensure_ascii=False).encode()
    out.update({
        "base": base,
        "annotations": annotations or {},
        "digests": {"schedule": "sha256:" + hashlib.sha256(schedule).hexdigest()},
        "week": week_runs,
        "transitions": transitions(week),
        # Message text is not pinned: the golden runner replaces it with "…" before comparing. Positioned warnings
        # are given as (code, marker[, nth]) and get line, column and excerpt in write(), which knows the input.
        "warnings": [{"code": w, "message": "…"} if isinstance(w, str) else {"code": w[0], "at": w[1:]}
                     for w in warnings],
    })
    return out


def pos(text, marker, nth=1):
    idx = -1
    for _ in range(nth):
        idx = text.index(marker, idx + 1)
    line = text.count("\n", 0, idx) + 1
    col = idx - (text.rfind("\n", 0, idx) + 1) + 1
    return f"{line}:{col}"


# Symbol presets (representation.ascii.symbols), assigned by state order in STATES, round robin.
PRESETS = {"ascii": [".", ":", "=", "#"], "braille": ["⠀", "⣀", "⣤", "⣶", "⣿"]}


def preview_rows(base, ranges, size=15, symbols="ascii"):
    """Reference downsampling: each bucket shows the state covering most of its minutes; ties go to the later state
    in STATES. Returns the day rows and the number of buckets that mix states."""
    week = paint_minutes(base, ranges)
    rows, mixed = [], 0
    for d, day in enumerate(DAYS):
        cells = ""
        for start in range(0, 1440, size):
            bucket = week[d * 1440 + start:d * 1440 + start + size]
            counts = {t: bucket.count(t) for t in set(bucket)}
            best = max(counts, key=lambda t: (counts[t], STATES.index(t)))
            mixed += len(counts) > 1
            preset = PRESETS[symbols]
            cells += preset[STATES.index(best) % len(preset)]
        rows.append(f"{day} {cells}")
    return rows, mixed


def write(name, md, args=(), want=None, diag=(), preview=None):
    d = os.path.join(OUT, name)
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "input.md"), "w") as f:
        f.write(md)
    if args:
        with open(os.path.join(d, "args"), "w") as f:
            f.write("\n".join(args) + "\n")
    if want is not None:
        for i, w in enumerate(want["warnings"]):
            if "at" in w:
                line, col = (int(x) for x in pos(md, *w["at"]).split(":"))
                text = md.split("\n")[line - 1]
                marker = "".join("\t" if ch == "\t" else " " for ch in text[:col - 1]) + "^"
                want["warnings"][i] = {"code": w["code"], "line": line, "column": col, "message": "…",
                                       "excerpt": {"text": text, "marker": marker}}
        with open(os.path.join(d, "want.json"), "w") as f:
            f.write(json.dumps(want, indent=2, ensure_ascii=False) + "\n")
    if preview is not None:
        with open(os.path.join(d, "want.preview"), "w") as f:
            f.write("\n".join(preview) + "\n")
    if diag:
        lines = []
        for sev, where, code in diag:
            lines.append(f"{sev} {where} {code}")
        with open(os.path.join(d, "want.diag"), "w") as f:
            f.write("\n".join(lines) + "\n")


META = "> [!SCHEDULE-META]\n> - schema-version: `1`\n"
WD = ["MON", "TUE", "WED", "THU", "FRI"]
WE = ["SAT", "SUN"]


def doc(*blocks, title="Schedule-Name"):
    head = f"# {title}\n\n" if title is not None else ""
    return head + "\n".join(b if b.endswith("\n") else b + "\n" for b in blocks)


# ---------------------------------------------------------------- valid cases

write(
    "valid/basic",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n"),
    want=output("Minimum", [("06:00", "22:00", WD, "Medium")]),
)

write(
    "valid/midnight-spill",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Maximum\n> - `22:00` to `02:00` FRI\n"),
    want=output("Minimum", [("22:00", "02:00", ["FRI"], "Maximum")]),
)

write(
    "valid/week-wrap",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Maximum\n> - `22:00` to `02:00` SUN\n"),
    want=output("Minimum", [("22:00", "02:00", ["SUN"], "Maximum")]),
)

write(
    "valid/start-equals-end-full-day",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Medium\n> - `08:00` to `08:00` MON\n"),
    want=output("Minimum", [("08:00", "08:00", ["MON"], "Medium")]),
)

write(
    "valid/end-at-midnight",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Medium\n> - `10:00` to `00:00` SAT,SUN\n"),
    want=output("Minimum", [("10:00", "00:00", WE, "Medium")]),
)

write(
    "valid/whole-day-midnight-to-midnight",
    doc(META, "> [!SCHEDULE-BASE] Off\n", "> [!SCHEDULE-STATE] Medium\n> - `00:00` to `00:00` WED\n"),
    want=output("Off", [("00:00", "00:00", ["WED"], "Medium")]),
)

write(
    "valid/weekday-wraparound-range",
    doc(META, "> [!SCHEDULE-BASE] Off\n", "> [!SCHEDULE-STATE] Medium\n> - `09:00` to `17:00` FRI-MON\n"),
    want=output("Off", [("09:00", "17:00", ["FRI", "SAT", "SUN", "MON"], "Medium")]),
)

write(
    "valid/weekday-whitespace",
    doc(
        META,
        "> [!SCHEDULE-BASE] Off\n",
        "> [!SCHEDULE-STATE] Medium\n> - `09:00` to `17:00` MON - FRI\n> - `10:00` to `14:00` SAT, SUN   \n",
    ),
    want=output("Off", [("09:00", "17:00", WD, "Medium"), ("10:00", "14:00", WE, "Medium")]),
)

write(
    "valid/weekday-mixed-list",
    doc(META, "> [!SCHEDULE-BASE] Off\n", "> [!SCHEDULE-STATE] Medium\n> - `09:00` to `17:00` MON-WED,FRI,SUN\n"),
    want=output("Off", [("09:00", "17:00", ["MON", "TUE", "WED", "FRI", "SUN"], "Medium")]),
)

overlap = [
    ("06:00", "22:00", WD, "Medium"),
    ("12:00", "14:00", ["WED"], "Maximum"),
    ("20:00", "23:00", ["WED"], "Off"),
]
write(
    "valid/overlap-later-wins",
    doc(
        META,
        "> [!SCHEDULE-BASE] Minimum\n",
        "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n",
        "> [!SCHEDULE-STATE] Maximum\n> - `12:00` to `14:00` WED\n",
        "> [!SCHEDULE-STATE] Off\n> - `20:00` to `23:00` WED\n",
    ),
    want=output("Minimum", overlap),
)

write(
    "valid/overlap-earlier-block-loses",
    doc(
        META,
        "> [!SCHEDULE-BASE] Minimum\n",
        "> [!SCHEDULE-STATE] Maximum\n> - `12:00` to `14:00` WED\n",
        "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n",
    ),
    want=output("Minimum", [("12:00", "14:00", ["WED"], "Maximum"), ("06:00", "22:00", WD, "Medium")]),
)

write(
    "valid/adjacent-runs-merge",
    doc(
        META,
        "> [!SCHEDULE-BASE] Minimum\n",
        "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `12:00` TUE\n> - `12:00` to `18:00` TUE\n",
    ),
    want=output("Minimum", [("06:00", "12:00", ["TUE"], "Medium"), ("12:00", "18:00", ["TUE"], "Medium")]),
)

write(
    "valid/range-equal-to-base-is-noop",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Minimum\n> - `06:00` to `12:00` TUE\n"),
    want=output("Minimum", [("06:00", "12:00", ["TUE"], "Minimum")]),
)

write(
    "valid/base-default-index-0",
    doc(META, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n"),
    want=output("Off", [("06:00", "22:00", WD, "Medium")]),
)

write(
    "valid/base-from-flag",
    doc(META, "> [!SCHEDULE-STATE] Maximum\n> - `06:00` to `22:00` MON-FRI\n"),
    args=["--base", "Medium"],
    want=output("Medium", [("06:00", "22:00", WD, "Maximum")]),
)

write(
    "valid/base-document-wins-over-flag",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Maximum\n> - `06:00` to `22:00` MON-FRI\n"),
    args=["--base", "Medium"],
    want=output("Minimum", [("06:00", "22:00", WD, "Maximum")]),
)

write(
    "valid/require-base-satisfied",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Maximum\n> - `06:00` to `22:00` MON-FRI\n"),
    args=["--require-base", "true"],
    want=output("Minimum", [("06:00", "22:00", WD, "Maximum")]),
)

write(
    "valid/case-insensitive-names",
    doc(
        "> [!schedule-meta]\n> - schema-version: `1`\n",
        "> [!Schedule-Base] MINIMUM\n",
        "> [!SCHEDULE-state] medium\n> - `06:00` to `22:00` MON-FRI\n",
    ),
    want=output("Minimum", [("06:00", "22:00", WD, "Medium")]),
)

write(
    "valid/prose-and-standard-alerts-ignored",
    doc(
        "Some introduction for humans.\n",
        "> [!NOTE]\n> `08:00` to `09:00` MON\n",
        META,
        "## States\n",
        "> [!TIP]\n> Later ranges win where they overlap.\n",
        "> [!SCHEDULE-BASE] Minimum\n",
        "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n",
        "A closing remark.\n",
    ),
    want=output("Minimum", [("06:00", "22:00", WD, "Medium")]),
)

write(
    "valid/annotations",
    doc(
        "> [!SCHEDULE-META]\n> - schema-version: `1`\n> - x-owner: `team-xyz`\n"
        "> - x-ticket: `OPS-1234`\n",
        "> [!SCHEDULE-BASE] Minimum\n",
        "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n",
    ),
    want=output(
        "Minimum",
        [("06:00", "22:00", WD, "Medium")],
        {"x-owner": "team-xyz", "x-ticket": "OPS-1234"},
    ),
)

write(
    "valid/sample-interval-60",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n"),
    args=["--sample-interval", "60"],
    want=output("Minimum", [("06:00", "22:00", WD, "Medium")]),
)

# A no-break space (U+00A0) counts as whitespace wherever the grammar allows whitespace: around the summary and the name,
# around "to", before and inside the weekday expression, and around the colon of META keys.
NBSP = "\u00a0"
# A state may appear in several STATE blocks; ranges paint in source order across blocks.
write(
    "valid/same-state-in-several-blocks",
    doc(
        META, "> [!SCHEDULE-BASE] Minimum\n",
        "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `10:00` MON\n",
        "> [!SCHEDULE-STATE] Maximum\n> - `08:00` to `09:00` MON\n",
        "> [!SCHEDULE-STATE] Medium\n> - `18:00` to `20:00` MON\n",
    ),
    want=output("Minimum", [("06:00", "10:00", ["MON"], "Medium"), ("08:00", "09:00", ["MON"], "Maximum"),
                            ("18:00", "20:00", ["MON"], "Medium")]),
)

write(
    "valid/nbsp-as-whitespace",
    doc(
        f"> [!SCHEDULE-META]{NBSP}Quiet days{NBSP}\n> - x-owner{NBSP}:{NBSP}`team-xyz`\n",
        "> [!SCHEDULE-BASE] Minimum\n",
        f"> [!SCHEDULE-STATE]{NBSP}Medium\n> - `06:00`{NBSP}to{NBSP}`22:00`{NBSP}MON{NBSP}-{NBSP}FRI\n",
        title=f"Office Hours{NBSP}",
    ),
    want=output("Minimum", [("06:00", "22:00", WD, "Medium")], {"x-owner": "team-xyz"}, name="Office Hours",
                summary="Quiet days"),
)

write(
    "valid/alert-permissive-namespace-typo",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDUEL-STATE] Medium\n> - `06:00` to `22:00` MON\n"),
    want=output("Minimum", []),
)

write(
    "valid/alert-pattern-standard-alerts",
    doc(
        META, "> [!SCHEDULE-BASE] Minimum\n", "> [!NOTE]\n> Read me.\n", "> [!tip]\n> Lowercase matches with (?i).\n",
        "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n",
    ),
    args=["--error-unless-alert-pattern", "(?i)NOTE|TIP|IMPORTANT|WARNING|CAUTION"],
    want=output("Minimum", [("06:00", "22:00", WD, "Medium")]),
)

write(
    "valid/sample-interval-1-odd-minutes",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Medium\n> - `06:07` to `21:53` MON\n"),
    want=output("Minimum", [("06:07", "21:53", ["MON"], "Medium")]),
)

write(
    "valid/time-single-digit-hour",
    doc(
        META,
        "> [!SCHEDULE-BASE] Minimum\n",
        "> [!SCHEDULE-STATE] Medium\n> - `6:00` to `9:30` MON\n> - `0:15` to `08:45` TUE\n",
    ),
    want=output("Minimum", [("6:00", "9:30", ["MON"], "Medium"), ("0:15", "08:45", ["TUE"], "Medium")]),
)

write(
    "valid/meta-omitted",
    doc("> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n"),
    want=output("Minimum", [("06:00", "22:00", WD, "Medium")]),
)

write(
    "valid/meta-summary",
    doc(
        "> [!SCHEDULE-META]   Office hours plus late Friday 2   \n> - schema-version: `1`\n",
        "> [!SCHEDULE-BASE] Minimum\n",
        "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n",
    ),
    want=output("Minimum", [("06:00", "22:00", WD, "Medium")], summary="Office hours plus late Friday 2"),
)

write(
    "valid/meta-summary-without-items",
    doc(
        "> [!SCHEDULE-META] Weekend shop\n",
        "> [!SCHEDULE-BASE] Off\n",
        "> [!SCHEDULE-STATE] Medium\n> - `10:00` to `18:00` SAT,SUN\n",
        title="Weekend-Shop 2026",
    ),
    want=output("Off", [("10:00", "18:00", WE, "Medium")], name="Weekend-Shop 2026", summary="Weekend shop"),
)

write(
    "valid/meta-default-schema-version",
    doc(
        "> [!SCHEDULE-META] Office hours\n> - x-owner: `team-xyz`\n",
        "> [!SCHEDULE-BASE] Minimum\n",
        "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n",
    ),
    want=output("Minimum", [("06:00", "22:00", WD, "Medium")], {"x-owner": "team-xyz"}, summary="Office hours"),
)

write(
    "valid/no-transitions-all-base",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Minimum\n> - `00:00` to `00:00` MON-SUN\n"),
    want=output("Minimum", [("00:00", "00:00", DAYS, "Minimum")]),
)

write(
    "valid/no-transitions-single-state",
    doc(META, "> [!SCHEDULE-BASE] Off\n", "> [!SCHEDULE-STATE] Medium\n> - `00:00` to `00:00` MON-SUN\n"),
    want=output("Off", [("00:00", "00:00", DAYS, "Medium")]),
)

write(
    "valid/transitions-every-day",
    doc(META, "> [!SCHEDULE-BASE] Off\n", "> [!SCHEDULE-STATE] Medium\n> - `8:15` to `17:45` MON-SUN\n"),
    want=output("Off", [("08:15", "17:45", DAYS, "Medium")]),
)

# ---------------------------------------------------------------- previews (want.preview pins the bucketed day rows)

basic_ranges = [("06:00", "22:00", WD, "Medium")]
rows, _ = preview_rows("Minimum", basic_ranges)
write(
    "valid/preview-default",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n"),
    args=["--output", "ascii"],
    want=output("Minimum", basic_ranges),
    preview=rows,
)

odd_ranges = [("06:07", "09:52", ["MON"], "Medium")]
odd_doc = doc(META, "> [!SCHEDULE-BASE] Off\n", "> [!SCHEDULE-STATE] Medium\n> - `06:07` to `09:52` MON\n")
rows, mixed = preview_rows("Off", odd_ranges)
assert mixed == 2
write(
    "valid/preview-approximate",
    odd_doc,
    args=["--output", "ascii"],
    want=output("Off", odd_ranges, warnings=["preview-approximate"]),
    diag=[("warn", "-", "preview-approximate")],
    preview=rows,
)

rows, mixed = preview_rows("Off", odd_ranges, size=1)
assert mixed == 0
write(
    "valid/preview-no-downsample",
    odd_doc,
    args=["--no-preview-downsample", "--output", "ascii"],
    want=output("Off", odd_ranges),
    preview=rows,
)

tie_ranges = [("23:00", "01:30", ["SUN"], "Maximum"), ("21:30", "05:30", WD, "Medium")]
rows, mixed = preview_rows("Off", tie_ranges, size=60)
write(
    "valid/preview-downsample-60-ties",
    doc(
        META,
        "> [!SCHEDULE-BASE] Off\n",
        "> [!SCHEDULE-STATE] Maximum\n> - `23:00` to `01:30` SUN\n",
        "> [!SCHEDULE-STATE] Medium\n> - `21:30` to `05:30` MON-FRI\n",
    ),
    args=["--preview-downsample=60", "--output", "ascii"],
    want=output("Off", tie_ranges, warnings=["preview-approximate"]),
    diag=[("warn", "-", "preview-approximate")],
    preview=rows,
)

rows, _ = preview_rows("Minimum", basic_ranges, symbols="braille")
write(
    "valid/preview-braille",
    doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n"),
    args=["--ascii-symbols", "braille", "--output", "ascii"],
    want=output("Minimum", basic_ranges),
    preview=rows,
)

write(
    "valid/preview-warning-only-with-preview",
    odd_doc,
    want=output("Off", odd_ranges),
)

# ---------------------------------------------------------------- warnings (valid output + diagnostics)

md = doc(META, "> [!SCHEDULE-BASE] Minimum\n", "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON-FRI\n")
write(
    "warn/fewer-styles-than-states",
    md,
    args=[
        "--set-config-at",
        "representation.styles.states",
        "--set-config-value",
        '[{"symbol": "a", "color": {"light": "#000000", "dark": "#ffffff"}},'
        ' {"symbol": "b", "color": {"light": "#111111", "dark": "#eeeeee"}}]',
    ],
    want=output("Minimum", [("06:00", "22:00", WD, "Medium")], warnings=["style-reuse"]),
    diag=[("warn", "-", "style-reuse")],
)

md = doc(META, "> [!SCHEDULE-BASE] Off\n", "> [!SCHEDULE-STATE] Medium\n> - `09:00` to `17:00` MON-WED,TUE\n")
write(
    "warn/repeated-weekday-overlap",
    md,
    want=output("Off", [("09:00", "17:00", ["MON", "TUE", "WED"], "Medium")],
                warnings=[("repeated-weekday", "TUE\n")]),
    diag=[("warn", pos(md, "TUE\n"), "repeated-weekday")],
)

md = doc(META, "> [!SCHEDULE-BASE] Off\n", "> [!SCHEDULE-STATE] Medium\n> - `09:00` to `17:00` FRI-MON,SUN\n")
write(
    "warn/repeated-weekday-wraparound",
    md,
    want=output("Off", [("09:00", "17:00", ["FRI", "SAT", "SUN", "MON"], "Medium")],
                warnings=[("repeated-weekday", "SUN\n")]),
    diag=[("warn", pos(md, "SUN\n"), "repeated-weekday")],
)

# ---------------------------------------------------------------- errors (no output)

def err(name, blocks, marker, code, args=(), nth=1):
    md = doc(*blocks)
    where = pos(md, marker, nth) if marker else "-"
    write(name, md, args=args, diag=[("error", where, code)])


BASE = "> [!SCHEDULE-BASE] Minimum\n"

err("error/time-24-00", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `10:00` to `24:00` SAT\n"], "24:00", "invalid-time")
err("error/time-minute-60", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `23:60` to `02:00` SAT\n"], "23:60", "invalid-time")
err("error/time-garbage", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `noon` to `14:00` SAT\n"], "noon", "invalid-time")
err("error/weekday-lowercase", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` mon-fri\n"], "mon", "invalid-weekday")
err("error/weekday-mixed-case", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` Mon\n"], "Mon", "invalid-weekday")
err("error/weekday-alias", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` weekdays\n"], "weekdays", "invalid-weekday")
err("error/weekday-trailing-dot", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON.\n"], "MON.", "invalid-weekday")
err("error/weekday-full-name", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MONDAY\n"], "MONDAY", "invalid-weekday")
err("error/weekday-missing", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00`\n"], "`06:00`", "invalid-range")
err("error/cron-form", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `0 8 * * MON-FRI`\n"], "`0 8", "invalid-range")
err("error/separator-dash", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00`-`22:00` MON\n"], "`06:00`-", "invalid-range")
err("error/separator-until", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00` until `22:00` MON\n"], "`06:00` until", "invalid-range")
err("error/prose-in-range", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - from `06:00` to `22:00` MON\n"], "from", "invalid-range")
err("error/range-not-in-list", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> `06:00` to `22:00` MON\n"], "`06:00` to", "invalid-range")
err("error/unknown-kind", [META, BASE, "> [!SCHEDULE-TEIR] Medium\n> - `06:00` to `22:00` MON\n"], "SCHEDULE-TEIR", "unknown-kind")
err("error/unknown-state", [META, BASE, "> [!SCHEDULE-STATE] Turbo\n> - `06:00` to `22:00` MON\n"], "Turbo", "unknown-state")
err("error/unknown-base-state", [META, "> [!SCHEDULE-BASE] Nope\n", "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON\n"], "Nope", "unknown-state")
err("error/state-title-missing", [META, BASE, "> [!SCHEDULE-STATE]\n> - `06:00` to `22:00` MON\n"], "SCHEDULE-STATE", "missing-title")
err("error/meta-wrong-schema-version", ["> [!SCHEDULE-META]\n> - schema-version: `2`\n", BASE], "2`", "meta-schema")
err("error/meta-unknown-key", ["> [!SCHEDULE-META]\n> - schema-version: `1`\n> - owner: `team-xyz`\n", BASE], "owner", "meta-schema")
err(
    "error/meta-custom-value-too-long",
    ["> [!SCHEDULE-META]\n> - schema-version: `1`\n> - x-note: `" + "x" * 49 + "`\n", BASE],
    "x" * 49,
    "meta-schema",
)
err(
    "error/meta-custom-key-uppercase",
    ["> [!SCHEDULE-META]\n> - schema-version: `1`\n> - x-Owner: `team-xyz`\n", BASE],
    "x-Owner",
    "meta-schema",
)
err(
    "error/sample-interval-mismatch",
    [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:10` to `22:00` MON\n"],
    "06:10",
    "sample-interval",
    args=["--sample-interval", "15"],
)
err("error/sample-interval-not-divisor-of-60", [META, BASE], None, "invalid-flag", args=["--sample-interval", "7"])
err("error/sample-interval-zero", [META, BASE], None, "invalid-flag", args=["--sample-interval", "0"])
err(
    "error/require-base-missing",
    [META, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON\n"],
    None,
    "missing-base",
    args=["--require-base", "true"],
)
err("error/require-base-with-base-flag", [META, BASE], None, "invalid-flag", args=["--require-base", "true", "--base", "Medium"])
err("error/base-flag-unknown-state", [META], None, "invalid-flag", args=["--base", "Turbo"])
err(
    "error/override-index-out-of-range",
    [META, BASE],
    None,
    "invalid-override",
    args=["--set-config-at", "states[9]", "--set-config-value", '{"id": "X", "display-name": "X"}'],
)
err(
    "error/override-unknown-path",
    [META, BASE],
    None,
    "invalid-override",
    args=["--set-config-at", "representation.colours", "--set-config-value", "{}"],
)
err(
    "error/override-unpaired",
    [META, BASE],
    None,
    "invalid-flag",
    args=["--set-config-at", "representation.theme"],
)
err(
    "error/override-invalid-json",
    [META, BASE],
    None,
    "invalid-override",
    args=["--set-config-at", "states[0].display-name", "--set-config-value", "Off"],
)
err(
    "error/override-violates-schema",
    [META, BASE],
    None,
    "config-schema",
    args=["--set-config-at", "representation.theme.light.background", "--set-config-value", '"red"'],
)
err(
    "error/override-duplicate-state-id",
    [META, BASE],
    None,
    "config-duplicate-state",
    args=["--set-config-at", "states[1]", "--set-config-value", '{"id": "off", "display-name": "Off again"}'],
)
# Other invisible characters are not whitespace: a zero-width space glues onto the weekday.
err("error/zero-width-space-in-weekdays", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON\u200b-FRI\n"], "MON", "invalid-weekday")

# The separator is a hyphen; the former colon form points to it.
err("error/marker-colon", [META, BASE, "> [!SCHEDULE:STATE] Medium\n> - `06:00` to `22:00` MON\n"], "SCHEDULE:STATE", "invalid-marker")

# Alerts other than schedule directives are prose, unless --error-unless-alert-pattern restricts them. The pattern is
# anchored by the tool; schedule directives are always allowed.
PATTERN = ["--error-unless-alert-pattern", "(?i)NOTE|TIP|IMPORTANT|WARNING|CAUTION"]
err("error/alert-pattern-namespace-typo", [META, BASE, "> [!SCHEDUEL-STATE] Medium\n> - `06:00` to `22:00` MON\n"], "SCHEDUEL-STATE", "disallowed-alert", args=PATTERN)
# The former aliases (CANNED-SCHEDULE, TO-SCHEDULE, CAN-SCHED, CONFIG-SCHED) were dropped on 2026-10-25.
err("error/alert-pattern-former-alias", [META, BASE, "> [!CAN-SCHED-STATE] Medium\n> - `06:00` to `22:00` MON\n"], "CAN-SCHED-STATE", "disallowed-alert", args=PATTERN)
err("error/alert-pattern-custom-callout", [META, BASE, "> [!custom-question] Why\n> Because.\n"], "custom-question", "disallowed-alert", args=PATTERN)
err("error/alert-pattern-anchored", [META, BASE, "> [!NOTE-extra]\n> Text.\n"], "NOTE-extra", "disallowed-alert", args=["--error-unless-alert-pattern", "NOTE"])
err("error/alert-pattern-invalid", [META, BASE], None, "invalid-flag", args=["--error-unless-alert-pattern", "(NOTE"])
err("error/time-single-digit-minute", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `6:0` to `9:30` MON\n"], "6:0`", "invalid-time")
err("error/time-three-digit-hour", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `006:00` to `9:30` MON\n"], "006:00", "invalid-time")

STATE = "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON\n"
err("error/missing-kind", [META, BASE, "> [!SCHEDULE] Medium\n> - `06:00` to `22:00` MON\n"], "SCHEDULE]", "missing-kind")
err("error/missing-kind-empty", [META, BASE, "> [!SCHEDULE-] Medium\n> - `06:00` to `22:00` MON\n"], "SCHEDULE-]", "missing-kind")
err("error/duplicate-base", [META, BASE, "> [!SCHEDULE-BASE] Medium\n", STATE], "SCHEDULE-BASE", "duplicate-block", nth=2)
err("error/duplicate-meta", [META, BASE, META, STATE], "SCHEDULE-META", "duplicate-block", nth=2)
err("error/empty-state", [META, BASE, "> [!SCHEDULE-STATE] Medium\n", STATE], "SCHEDULE-STATE", "empty-state")
err(
    "error/misplaced-directive-in-list",
    [META, BASE, "- a list item\n\n  > [!SCHEDULE-STATE] Medium\n  > - `06:00` to `22:00` MON\n"],
    "SCHEDULE-STATE",
    "misplaced-directive",
)
err(
    "error/misplaced-directive-bold",
    [META, BASE, "> **[!SCHEDULE-STATE]** Medium\n> - `06:00` to `22:00` MON\n"],
    "SCHEDULE-STATE",
    "misplaced-directive",
)
err("error/summary-too-long", ["> [!SCHEDULE-META] " + "a" * 81 + "\n", BASE, STATE], "a" * 81, "invalid-summary")
err("error/summary-bad-character", ["> [!SCHEDULE-META] Office hours!\n", BASE, STATE], "Office", "invalid-summary")
err("error/summary-trailing-hyphen", ["> [!SCHEDULE-META] Office-\n", BASE, STATE], "Office", "invalid-summary")
err("error/summary-leading-digit", ["> [!SCHEDULE-META] 24h service\n", BASE, STATE], "24h", "invalid-summary")
err("error/summary-on-base", [META, "> [!SCHEDULE-BASE] Minimum extra\n", STATE], "Minimum extra", "unknown-state")

def err_doc(name, md, marker, code, nth=1):
    write(name, md, diag=[("error", pos(md, marker, nth) if marker else "-", code)])

err_doc("error/missing-name", doc(META, BASE, STATE, title=None), None, "missing-name")
err_doc("error/name-leading-digit", doc(META, BASE, STATE, title="2026 Plan"), "2026", "invalid-name")
err_doc("error/name-bad-character", doc(META, BASE, STATE, title="Plan: Q1"), "Plan", "invalid-name")
err_doc("error/name-too-long", doc(META, BASE, STATE, title="N" * 81), "N" * 81, "invalid-name")
err_doc("error/duplicate-name", doc(META, BASE, STATE, "# Second-Name\n"), "Second-Name", "duplicate-name")
write(
    "warn/override-duplicate-symbol",
    doc(META, BASE, STATE),
    args=["--ascii-symbols", "styles", "--set-config-at", "representation.styles.states[1].symbol",
          "--set-config-value", '"."'],
    want=output("Minimum", [("06:00", "22:00", ["MON"], "Medium")], warnings=["config-duplicate-symbol"]),
    diag=[("warn", "-", "config-duplicate-symbol")],
)
err("error/report-format-unknown", [META, BASE, STATE], None, "invalid-flag", args=["--error-output-format", "yaml"])
err("error/report-template-unknown", [META, BASE, STATE], None, "invalid-flag", args=["--error-report-template", "html"])
err(
    "error/report-format-and-template",
    [META, BASE, STATE],
    None,
    "invalid-flag",
    args=["--error-output-format", "json", "--error-report-template", "markdown-gfm"],
)
err("error/preview-downsample-not-divisor-of-60", [META, BASE, STATE], None, "invalid-flag", args=["--preview-downsample=7"])
err("error/preview-downsample-not-a-number", [META, BASE, STATE], None, "invalid-flag", args=["--preview-downsample=x"])
err(
    "error/preview-downsample-conflict",
    [META, BASE, STATE],
    None,
    "invalid-flag",
    args=["--preview-downsample=30", "--no-preview-downsample"],
)
err("error/output-unknown", [META, BASE, STATE], None, "invalid-flag", args=["--output", "yaml"])
err("error/color-unknown", [META, BASE, STATE], None, "invalid-flag", args=["--color", "sometimes"])
err("error/ascii-symbols-unknown", [META, BASE, STATE], None, "invalid-flag", args=["--ascii-symbols", "blocks"])
err(
    "error/ascii-symbols-config-unknown",
    [META, BASE, STATE],
    None,
    "config-schema",
    args=["--set-config-at", "representation.ascii.symbols", "--set-config-value", '"blocks"'],
)
err("error/duplicate-weekday-list", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` MON,MON\n"], "MON", "duplicate-weekday", nth=2)
err("error/duplicate-weekday-one-day-range", [META, BASE, "> [!SCHEDULE-STATE] Medium\n> - `06:00` to `22:00` WED-WED\n"], "WED", "duplicate-weekday", nth=2)
err("error/svg-theme-unknown", [META, BASE, STATE], None, "invalid-flag", args=["--svg-theme", "sepia"])
