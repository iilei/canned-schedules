resource "aws_appautoscaling_target" "app" {
  service_namespace  = "ecs"
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.app.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  min_capacity       = local.capacity[local.schedule.base].min
  max_capacity       = local.capacity[local.schedule.base].max

  # The scheduled actions move min and max; without this, every apply would reset them to
  # the base state's values.
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
