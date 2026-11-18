# An imaginary service for the recipes in ec2.tf and ecs.tf to schedule. Replace it with your own infrastructure.
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
  description = "The AWS region of the imaginary service."
  type        = string
  default     = "eu-central-1"
}

variable "image_id" {
  description = "The AMI the Auto Scaling group starts."
  type        = string
}

resource "aws_vpc" "main" {
  cidr_block = "10.42.0.0/16"
}

resource "aws_subnet" "app" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.42.1.0/24"
  availability_zone = "${var.region}a"
}

resource "aws_launch_template" "app" {
  name_prefix   = "imaginary-"
  image_id      = var.image_id
  instance_type = "t3.micro"
}

resource "aws_autoscaling_group" "app" {
  name                = "imaginary-service"
  min_size            = 0
  max_size            = 0
  desired_capacity    = 0
  vpc_zone_identifier = [aws_subnet.app.id]

  launch_template {
    id      = aws_launch_template.app.id
    version = "$Latest"
  }

  # The scheduled actions move these values; without this, every apply would reset them.
  lifecycle {
    ignore_changes = [min_size, max_size, desired_capacity]
  }
}

resource "aws_ecs_cluster" "main" {
  name = "imaginary"
}

resource "aws_ecs_task_definition" "app" {
  family = "imaginary-service"
  container_definitions = jsonencode([{
    name      = "app"
    image     = "example.invalid/app:1"
    memory    = 128
    essential = true
  }])
}

resource "aws_ecs_service" "app" {
  name            = "imaginary-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.app.arn
  desired_count   = 0

  # Application Auto Scaling owns the desired count once the service is a scalable target.
  lifecycle {
    ignore_changes = [desired_count]
  }
}
