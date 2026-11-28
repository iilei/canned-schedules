locals {
  schedule = jsondecode(file("${path.module}/office-hours.canned-schedule.json"))
  capacity = jsondecode(file("${path.module}/capacity.json"))
  transitions = {
    for t in local.schedule.transitions :
    "${replace(t.time, ":", "")}-${join("_", t.days)}-${t.state}" => t
  }
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
