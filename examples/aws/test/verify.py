#!/usr/bin/env python3
"""Checks for mise run example:aws. Standard library only; AWS is read through the aws CLI."""
import argparse
from datetime import datetime, timezone
import glob
import json
import os
import re
import subprocess
import sys

DAYS = ["MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"]
WEEK_MINUTES = 7 * 24 * 60
DAY_MINUTES = 24 * 60
SUNDAY_FIRST = ["SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"]
TIME_ZONE = "Etc/UTC"
PLAN_MINUTES = 10


def fail(message):
    sys.exit(f"FAIL: {message}")


def ok(message):
    print(f"ok: {message}", flush=True)


def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def aws(*args):
    out = subprocess.run(["aws", *args, "--output", "json"], capture_output=True, text=True, check=False)
    if out.returncode != 0:
        fail(f"aws {' '.join(args)}: {out.stderr.strip()}")
    return json.loads(out.stdout)


def action_name(transition):
    """The name the recipes give a transition's action."""
    time = transition["time"].replace(":", "")
    return f"office-hours-{time}-{'_'.join(transition['days'])}-{transition['state']}"


def days_of(field, numeric):
    """The weekdays of a cron field: numbers with 0 = SUN, or names."""
    if field in ("*", "?"):
        return set(DAYS)
    names = [str(i) for i in range(7)] if numeric else SUNDAY_FIRST
    picked = set()
    for part in field.split(","):
        first, _, last = part.partition("-")
        start, end = names.index(first), names.index(last or first)
        picked.update(SUNDAY_FIRST[start : end + 1])
    return picked


def firing(cron):
    """When a five-field (posix) or six-field (AWS) cron fires: minute of the day and weekdays."""
    fields = cron.split()
    if len(fields) == 5:
        return int(fields[1]) * 60 + int(fields[0]), days_of(fields[4], numeric=True)
    if len(fields) == 6 and fields[2] == "?" and fields[3] == "*" and fields[5] == "*":
        return int(fields[1]) * 60 + int(fields[0]), days_of(fields[4], numeric=False)
    fail(f"unexpected cron: {cron}")


def minute(text):
    return int(text[:2]) * 60 + int(text[3:])


def painted(week):
    """The state of every minute of the week, Monday 00:00 first."""
    states = [None] * WEEK_MINUTES
    for index, day in enumerate(DAYS):
        for run in week[day]:
            for m in range(minute(run["start"]), minute(run["end"]) or DAY_MINUTES):
                states[index * DAY_MINUTES + m] = run["state"]
    return states


def replay(actions, start):
    """The state of every minute when each (cron, state) action fires at its times, week after week."""
    fires = {}
    for cron, state in actions:
        at, days = firing(cron)
        for day in days:
            fires[DAYS.index(day) * DAY_MINUTES + at] = state
    current, states = start, []
    for _ in range(2):  # The second pass starts with the state the week ended in.
        states = []
        for m in range(WEEK_MINUTES):
            current = fires.get(m, current)
            states.append(current)
    return states


def golden(args):
    outputs = sorted(glob.glob(os.path.join(args.dir, "*", "*", "want.json")))
    checked = 0
    for path in outputs:
        doc = load(path)
        if not doc["transitions"]:
            continue
        checked += 1
        week = painted(doc["week"])
        for dialect in ("posix", "aws"):
            actions = [(t["cron"][dialect], t["state"]) for t in doc["transitions"]]
            if replay(actions, week[-1]) != week:
                fail(f"{path}: the {dialect} crons don't reproduce the week")
    if not checked:
        fail(f"no golden outputs in {args.dir}")
    ok(f"both cron formats reproduce the week in {checked} golden outputs")


def check_week(label, doc, actions):
    if actions and replay(actions, doc["base"]) != painted(doc["week"]):
        fail(f"{label}: replaying the stored crons doesn't reproduce the week")


def ec2(args):
    doc, capacity = load(args.schedule), load(args.capacity)
    want = {action_name(t): t for t in doc["transitions"]}
    listed = aws("autoscaling", "describe-scheduled-actions", "--auto-scaling-group-name", args.group)
    got = {a["ScheduledActionName"]: a for a in listed["ScheduledUpdateGroupActions"]}
    if args.new_only:
        got = {name: action for name, action in got.items() if name in want}
    if set(got) != set(want):
        fail(f"{args.group}: actions differ: {sorted(set(got) ^ set(want))}")
    states = {(c["min"], c["desired"], c["max"]): state for state, c in capacity.items()}
    actions = []
    for name, t in want.items():
        a, c = got[name], capacity[t["state"]]
        stored = (a.get("MinSize"), a.get("DesiredCapacity"), a.get("MaxSize"))
        if a.get("Recurrence") != t["cron"]["posix"] or a.get("TimeZone") != TIME_ZONE or stored != (
            c["min"], c["desired"], c["max"]
        ):
            fail(f"{name}: stored as {a}")
        actions.append((a["Recurrence"], states[stored]))
    check_week(args.group, doc, actions)
    ok(f"{args.group}: {len(want)} EC2 Auto Scaling actions match the schedule")


def ecs(args):
    doc, capacity = load(args.schedule), load(args.capacity)
    want = {action_name(t): t for t in doc["transitions"]}
    listed = aws("application-autoscaling", "describe-scheduled-actions", "--service-namespace", "ecs",
                 "--resource-id", args.resource)
    got = {a["ScheduledActionName"]: a for a in listed["ScheduledActions"]}
    if set(got) != set(want):
        fail(f"{args.resource}: actions differ: {sorted(set(got) ^ set(want))}")
    states = {(c["min"], c["max"]): state for state, c in capacity.items()}
    actions = []
    for name, t in want.items():
        a, c = got[name], capacity[t["state"]]
        target = a["ScalableTargetAction"]
        stored = (target.get("MinCapacity"), target.get("MaxCapacity"))
        if a["Schedule"] != f"cron({t['cron']['aws']})" or a.get("Timezone") != TIME_ZONE or stored != (
            c["min"], c["max"]
        ):
            fail(f"{name}: stored as {a}")
        actions.append((t["cron"]["aws"], states[stored]))
    check_week(args.resource, doc, actions)
    ok(f"{args.resource}: {len(want)} Application Auto Scaling actions match the schedule")


def plan(args):
    old = {action_name(t) for t in load(args.old)["transitions"]}
    new = {action_name(t) for t in load(args.new)["transitions"]}
    names = {"aws_autoscaling_schedule": "scheduled_action_name", "aws_appautoscaling_scheduled_action": "name"}
    changes = [c for c in load(args.plan)["resource_changes"] if c["change"]["actions"] != ["no-op"]]
    others = [c["address"] for c in changes if c["type"] not in names]
    if others:
        fail(f"the plan also changes {others}")
    for kind, key in names.items():
        removed = {c["change"]["before"][key] for c in changes if c["type"] == kind and "delete" in c["change"]["actions"]}
        added = {c["change"]["after"][key] for c in changes if c["type"] == kind and "create" in c["change"]["actions"]}
        if removed != old or added != new:
            fail(f"{kind}: the plan removes {sorted(removed)} and adds {sorted(added)}")
    ok("the plan replaces exactly the old actions with the new ones")


def deletes(args):
    with open(args.log, encoding="utf-8") as f:
        got = {re.search(r"--scheduled-action-name (\S+)", line).group(1) for line in f}
    want = {action_name(t) for t in load(args.schedule)["transitions"]}
    if got != want:
        fail(f"the recipe deleted {sorted(got)}, want {sorted(want)}")
    ok(f"the recipe deletes exactly the {len(want)} old actions")


def shifts_of(doc):
    """The shift starts as listed in occurrences, as (minute of the week, point in time)."""
    shifts = []
    for o in doc["occurrences"]:
        at = DAYS.index(o["dow"]) * DAY_MINUTES + minute(o["time"])
        shifts.append((at, {"id": o["state"], "dow": o["dow"], "time": o["time"]}))
    expanded = sorted((DAYS.index(d) * DAY_MINUTES + minute(t["time"]), t["state"]) for t in doc["transitions"] for d in t["days"])
    if [(at, point["id"]) for at, point in shifts] != expanded:
        fail("occurrences are not the transitions, one weekday each, in the order of the week")
    return shifts


def answer(shifts, position):
    """The parameter's value during one shift. The week repeats, so the ends of the list are neighbours."""
    return {
        "on_call_previous": shifts[position - 1][1],
        "on_call_current": shifts[position][1],
        "on_call_next": shifts[(position + 1) % len(shifts)][1],
    }


def position_at(shifts, at):
    """The index of the shift in effect at a minute of the week."""
    started = [i for i, shift in enumerate(shifts) if shift[0] <= at]
    return started[-1] if started else len(shifts) - 1


def schedules_of(group):
    listed = aws("scheduler", "list-schedules", "--group-name", group)["Schedules"]
    return {s["Name"]: aws("scheduler", "get-schedule", "--group-name", group, "--name", s["Name"]) for s in listed}


def parameter_value(name):
    return aws("ssm", "get-parameter", "--name", name)["Parameter"]["Value"]


def oncall(args):
    doc = load(args.schedule)
    shifts = shifts_of(doc)
    week = painted(doc["week"])
    for at, state in enumerate(week):
        if shifts[position_at(shifts, at)][1]["id"] != state:
            fail(f"{args.group}: the shifts don't match the week at minute {at}")

    want = {}
    for position, (_, point) in enumerate(shifts):
        name = f"{point['time'].replace(':', '')}-{point['dow']}-{point['id']}"
        want[name] = (point, answer(shifts, position), doc["occurrences"][position]["cron"]["aws"])

    got = schedules_of(args.group)
    if set(got) != set(want):
        fail(f"{args.group}: schedules differ: {sorted(set(got) ^ set(want))}")
    for name, (point, value, cron) in want.items():
        s = got[name]
        at, days = firing(cron)
        if at != minute(point["time"]) or days != {point["dow"]}:
            fail(f"{name}: {cron} doesn't fire at {point['dow']} {point['time']}")
        cron = f"cron({cron})"
        target = s["Target"]
        given = json.loads(target["Input"])
        if s["ScheduleExpression"] != cron or s.get("ScheduleExpressionTimezone") != TIME_ZONE:
            fail(f"{name}: runs at {s['ScheduleExpression']} in {s.get('ScheduleExpressionTimezone')}")
        if s["FlexibleTimeWindow"]["Mode"] != "OFF" or target["Arn"] != "arn:aws:scheduler:::aws-sdk:ssm:putParameter":
            fail(f"{name}: unexpected window or target: {s}")
        if (given["Name"], given["Type"], given["Overwrite"]) != (args.parameter, "String", True):
            fail(f"{name}: writes {given}")
        if json.loads(given["Value"]) != value:
            fail(f"{name}: value {given['Value']}, want {json.dumps(value)}")

    now = datetime.now(timezone.utc)
    current = now.weekday() * DAY_MINUTES + now.hour * 60 + now.minute
    # The plan ran moments ago, and a shift may have started since.
    accepted = [answer(shifts, position_at(shifts, (current - back) % WEEK_MINUTES)) for back in range(PLAN_MINUTES)]
    deployed = json.loads(parameter_value(args.parameter))
    if deployed not in accepted:
        fail(f"{args.parameter}: holds {deployed}, want {accepted[0]}")
    ok(f"{args.group}: {len(want)} shift starts carry the right answers, and {args.parameter} shows the current shift")


def oncall_fire(args):
    """Does what each schedule would do at its time, because the emulator doesn't run them."""
    got = schedules_of(args.group)
    for name, schedule in got.items():
        given = json.loads(schedule["Target"]["Input"])
        aws("ssm", "put-parameter", "--name", given["Name"], "--type", given["Type"], "--value", given["Value"],
            "--overwrite")
        if parameter_value(args.parameter) != given["Value"]:
            fail(f"{name}: {args.parameter} doesn't hold the schedule's value")
    ok(f"{args.group}: writing each of the {len(got)} schedules' inputs updates {args.parameter}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(required=True)

    sub = commands.add_parser("golden")
    sub.add_argument("dir")
    sub.set_defaults(run=golden)

    sub = commands.add_parser("ec2")
    sub.add_argument("--group", required=True)
    sub.add_argument("--schedule", required=True)
    sub.add_argument("--capacity", required=True)
    sub.add_argument("--new-only", action="store_true", help="ignore actions the emulator couldn't delete")
    sub.set_defaults(run=ec2)

    sub = commands.add_parser("ecs")
    sub.add_argument("--resource", required=True)
    sub.add_argument("--schedule", required=True)
    sub.add_argument("--capacity", required=True)
    sub.set_defaults(run=ecs)

    sub = commands.add_parser("plan")
    sub.add_argument("--plan", required=True)
    sub.add_argument("--old", required=True)
    sub.add_argument("--new", required=True)
    sub.set_defaults(run=plan)

    sub = commands.add_parser("deletes")
    sub.add_argument("--log", required=True)
    sub.add_argument("--schedule", required=True)
    sub.set_defaults(run=deletes)

    sub = commands.add_parser("oncall")
    sub.add_argument("--group", required=True)
    sub.add_argument("--parameter", required=True)
    sub.add_argument("--schedule", required=True)
    sub.set_defaults(run=oncall)

    sub = commands.add_parser("oncall-fire")
    sub.add_argument("--group", required=True)
    sub.add_argument("--parameter", required=True)
    sub.set_defaults(run=oncall_fire)

    args = parser.parse_args()
    args.run(args)


if __name__ == "__main__":
    main()
