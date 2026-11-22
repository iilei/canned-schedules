# AWS Auto Scaling scheduled actions

:::{warning}
Unverified against AWS. The recipes below are tested against an AWS emulator
([`examples/aws`](https://github.com/iilei/canned-schedules/tree/master/examples/aws), `mise run example:aws`), but neither has created real scheduled actions yet.
:::

[`examples/aws`](https://github.com/iilei/canned-schedules/tree/master/examples/aws) has these recipes as working files. Each transition becomes one scheduled action: `cron.posix` is its recurrence, and the capacity comes from a map you
keep next to your infrastructure, keyed by state id:

```json
{
  "Off":     {"min": 0, "desired": 0, "max": 0},
  "Minimum": {"min": 1, "desired": 1, "max": 2},
  "Medium":  {"min": 2, "desired": 4, "max": 6},
  "Maximum": {"min": 4, "desired": 8, "max": 12}
}
```

With Terraform, `for_each` over the transitions, keyed by time, days and state (`0600-MON_TUE_WED_THU_FRI-Medium`), so
the key says what the action does, and Terraform removes the actions of transitions that disappear:

```hcl
locals {
  schedule = jsondecode(file("${path.module}/office-hours.canned-schedule.json"))
  capacity = jsondecode(file("${path.module}/capacity.json"))
  transitions = { for t in local.schedule.transitions : "${replace(t.time, ":", "")}-${join("_", t.days)}-${t.state}" => t }
}

resource "aws_autoscaling_schedule" "office_hours" {
  for_each = local.transitions

  autoscaling_group_name = aws_autoscaling_group.app.name
  scheduled_action_name  = "office-hours-${each.key}"
  recurrence             = each.value.cron.posix
  time_zone              = "Etc/UTC"
  min_size               = local.capacity[each.value.state].min
  desired_capacity       = local.capacity[each.value.state].desired
  max_size               = local.capacity[each.value.state].max
}
```

With the AWS CLI (bash and jq), replace the schedule's actions in one go:

```bash
set -euo pipefail

asg=my-asg
prefix=office-hours
schedule=office-hours.canned-schedule.json

# One action per transition: its cron as the recurrence, the state's capacity from capacity.json. Computed before
# anything changes, so a state without a capacity stops the run while the group still has its old schedule.
actions=$(jq -r --slurpfile cap capacity.json --arg prefix "$prefix" '
  .transitions[]
  | ($cap[0][.state] // error("capacity.json has no entry for state \(.state)")) as $c
  | [$prefix + "-" + (.time | sub(":"; "")) + "-" + (.days | join("_")) + "-" + .state, .cron.posix, $c.min,
     $c.desired, $c.max]
  | @tsv' "$schedule")

# Remove this schedule's earlier actions, so a transition that was dropped from the schedule stops firing.
aws autoscaling describe-scheduled-actions --auto-scaling-group-name "$asg" --output json \
  --query "ScheduledUpdateGroupActions[?starts_with(ScheduledActionName, '$prefix-')].ScheduledActionName" |
  jq -r '.[]' |
  while read -r name; do
    aws autoscaling delete-scheduled-action --auto-scaling-group-name "$asg" --scheduled-action-name "$name"
  done

while IFS=$'	' read -r name cron min desired max; do
  [ -n "$name" ] || continue
  aws autoscaling put-scheduled-update-group-action --auto-scaling-group-name "$asg" \
    --scheduled-action-name "$name" --recurrence "$cron" --time-zone Etc/UTC \
    --min-size "$min" --desired-capacity "$desired" --max-size "$max"
done <<< "$actions"
```

- **The time zone is yours to choose.** The tool writes wall-clock times without a zone. `Etc/UTC` matches the
  recommended convention; with a zone such as `Europe/Berlin`, Auto Scaling follows its daylight saving changes.
- **Capacity until the next transition:** scheduled actions only fire at their times. After the first deployment the
  group keeps its current capacity until the next transition, so set it to the state in effect now (from `week`).
  With Terraform, also add `lifecycle { ignore_changes = [min_size, max_size, desired_capacity] }` to the group, or
  every apply resets what the actions set.
- **No transitions** (one state all week): there are no actions to create. Give the group the base state's capacity
  directly.
- **Limits:** Auto Scaling allows 125 scheduled actions per group, which is far more than a weekly schedule needs.
- **Names are yours to define.** The output has no id field on purpose: what makes two actions "the same" depends on
  your setup, and a field whose meaning could change would be the riskiest part of the contract. The key above,
  `<HHMM>-<days>-<state>`, carries every fact of the transition, so it is unique within one schedule and readable in
  the console. Changing a transition's time, days or state gives it a new name, so the action is replaced. To update
  actions in place when only their days change, leave the days out (`<HHMM>-<state>` is still unique within one
  schedule, since transitions are grouped by time and state).
- **Several schedules in one group:** keys are unique within one schedule only. Two schedules with a `Minimum` on
  Friday at `22:00` both give `2200-FRI-Minimum`. Action names only need to be unique per group, so one schedule per
  group is fine; when several schedules share a group or one `for_each` map (where `merge()` would silently keep only
  one of them), put the schedule's name into the key, e.g. `maintenance-2200-FRI-Minimum`.

## ECS services and other Application Auto Scaling targets

Application Auto Scaling (ECS services, DynamoDB capacity, Aurora replicas, Lambda provisioned concurrency, …) takes
the six-field format, so use `cron.aws` there, wrapped as `cron(...)`. Its actions set min and max only; there is no
desired count. The target has to be registered before its scheduled actions, here with the base state's capacity:

```hcl
resource "aws_appautoscaling_target" "app" {
  service_namespace  = "ecs"
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.app.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  min_capacity       = local.capacity[local.schedule.base].min
  max_capacity       = local.capacity[local.schedule.base].max

  # The scheduled actions move min and max; without this, every apply would reset them to the base state's values.
  lifecycle {
    ignore_changes = [min_capacity, max_capacity]
  }
}

resource "aws_appautoscaling_scheduled_action" "office_hours" {
  for_each = local.transitions

  name               = "office-hours-${each.key}"
  service_namespace  = aws_appautoscaling_target.app.service_namespace
  resource_id        = aws_appautoscaling_target.app.resource_id
  scalable_dimension = aws_appautoscaling_target.app.scalable_dimension
  schedule           = "cron(${each.value.cron.aws})"
  timezone           = "Etc/UTC"

  scalable_target_action {
    min_capacity = local.capacity[each.value.state].min
    max_capacity = local.capacity[each.value.state].max
  }
}
```

Add `lifecycle { ignore_changes = [desired_count] }` to the `aws_ecs_service`: Application Auto Scaling owns that value
once the service is a scalable target, and every apply would otherwise reset it.

`local.transitions` and `local.capacity` are the ones from the EC2 example. With the AWS CLI, the same values go to
`aws application-autoscaling put-scheduled-action` (`--schedule "cron(...)" --timezone Etc/UTC
--scalable-target-action MinCapacity=…,MaxCapacity=…`).
