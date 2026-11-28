#!/usr/bin/env bash
set -euo pipefail

asg=my-asg
prefix=office-hours
schedule=office-hours.canned-schedule.json

# One action per transition: its cron as the recurrence, the state's capacity from
# capacity.json. Computed before anything changes, so a state without a capacity stops the
# run while the group still has its old schedule.
actions=$(jq -r --slurpfile cap capacity.json --arg prefix "$prefix" '
  .transitions[]
  | ($cap[0][.state] // error("capacity.json has no entry for state \(.state)")) as $c
  | [$prefix + "-" + (.time | sub(":"; "")) + "-" + (.days | join("_")) + "-" + .state,
     .cron.posix, $c.min, $c.desired, $c.max]
  | @tsv' "$schedule")

# Remove this schedule's earlier actions, so a transition that was dropped from the
# schedule stops firing.
aws autoscaling describe-scheduled-actions --auto-scaling-group-name "$asg" \
  --output json |
  jq -r --arg p "$prefix-" '
    .ScheduledUpdateGroupActions[].ScheduledActionName | select(startswith($p))' |
  while read -r name; do
    aws autoscaling delete-scheduled-action --auto-scaling-group-name "$asg" \
      --scheduled-action-name "$name"
  done

while IFS=$'	' read -r name cron min desired max; do
  [ -n "$name" ] || continue
  aws autoscaling put-scheduled-update-group-action --auto-scaling-group-name "$asg" \
    --scheduled-action-name "$name" --recurrence "$cron" --time-zone Etc/UTC \
    --min-size "$min" --desired-capacity "$desired" --max-size "$max"
done <<< "$actions"
