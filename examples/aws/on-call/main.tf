# Who is on call right now, since when, and who takes over next?
#
# One EventBridge schedule per shift start writes the answer to one SSM parameter. There is no code to run: every schedule
# carries its ready-made value. A schedule describes one repeating week, so Sunday's last shift hands over to Monday's first.
terraform {
  required_version = ">= 1.15"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.72.0"
    }
  }
}

provider "aws" {
  region = var.region
}

variable "region" {
  description = "The AWS region of the schedules and the parameter."
  type        = string
  default     = "eu-central-1"
}

variable "schedule_file" {
  description = "The rendered schedule, next to this file. Each state is a person."
  type        = string
  default     = "on-call.canned-schedule.json"
}

variable "parameter_name" {
  description = "The SSM parameter that holds the answer."
  type        = string
  default     = "/on-call/current"
}

variable "schedule_group" {
  description = "The EventBridge Scheduler group of the shift starts."
  type        = string
  default     = "on-call"
}

locals {
  schedule = jsondecode(file("${path.module}/${var.schedule_file}"))
  weekdays = ["MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"]

  # One entry per shift start, in the order of the week.
  occurrences = local.schedule.occurrences
  total       = length(local.occurrences)
  points      = [for occurrence in local.occurrences : { id = occurrence.state, dow = occurrence.dow, time = occurrence.time }]

  # The week repeats, so the first shift follows the last one.
  answers = [
    for position, point in local.points : {
      on_call_previous = local.points[(position + local.total - 1) % local.total]
      on_call_current  = point
      on_call_next     = local.points[(position + 1) % local.total]
    }
  ]

  # The shift in effect while Terraform plans, so the parameter is right as soon as it is deployed.
  now        = plantimestamp()
  now_minute = index(local.weekdays, upper(formatdate("EEE", local.now))) * 1440 + tonumber(formatdate("h", local.now)) * 60 + tonumber(formatdate("m", local.now))
  started = [
    for position, point in local.points : position
    if index(local.weekdays, point.dow) * 1440 + tonumber(substr(point.time, 0, 2)) * 60 + tonumber(substr(point.time, 3, 2)) <= local.now_minute
  ]
  in_effect = length(local.started) > 0 ? local.started[length(local.started) - 1] : local.total - 1
}

resource "aws_ssm_parameter" "on_call" {
  name  = var.parameter_name
  type  = "String"
  value = jsonencode(try(local.answers[local.in_effect], {}))

  lifecycle {
    precondition {
      condition     = local.total > 0
      error_message = "The schedule has no transitions, so nobody ever takes over."
    }
  }
}

resource "aws_iam_role" "scheduler" {
  name = "${var.schedule_group}-scheduler"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { Service = "scheduler.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "scheduler" {
  name = "write-the-on-call-parameter"
  role = aws_iam_role.scheduler.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "ssm:PutParameter"
      Resource = aws_ssm_parameter.on_call.arn
    }]
  })
}

resource "aws_scheduler_schedule_group" "on_call" {
  name = var.schedule_group
}

resource "aws_scheduler_schedule" "shift" {
  for_each = {
    for position, point in local.points : "${replace(point.time, ":", "")}-${point.dow}-${point.id}" => position
  }

  name                         = each.key
  group_name                   = aws_scheduler_schedule_group.on_call.name
  schedule_expression          = "cron(${local.occurrences[each.value].cron.aws})"
  schedule_expression_timezone = "Etc/UTC"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = "arn:aws:scheduler:::aws-sdk:ssm:putParameter"
    role_arn = aws_iam_role.scheduler.arn

    input = jsonencode({
      Name      = aws_ssm_parameter.on_call.name
      Type      = "String"
      Overwrite = true
      Value     = jsonencode(local.answers[each.value])
    })
  }
}
