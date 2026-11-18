# Canned schedules on AWS

Working versions of the recipes in the main README's
[AWS section](../../README.md#aws-auto-scaling-scheduled-actions): an imaginary service whose capacity follows a schedule.

| Path                          | What it does                                                 |
| ----------------------------- | ------------------------------------------------------------ |
| `schedules/`                  | Two example schedules                                        |
| `capacity.json`               | The capacity of each state, kept next to your infrastructure |
| `terraform/ec2.tf`            | EC2 Auto Scaling scheduled actions, one per transition       |
| `terraform/ecs.tf`            | ECS service scaling through Application Auto Scaling         |
| `terraform/infrastructure.tf` | The imaginary service; replace it with your own              |
| `cli/apply-schedule.sh`       | The EC2 recipe with the AWS CLI and `jq`                     |
| `test/`                       | What `mise run example:aws` uses                             |

## Try it

Render a schedule next to the Terraform files, then copy the capacity map:

```sh
canned-schedules --json-output-file terraform/office-hours.canned-schedule.json \
  schedules/friday-peak.canned-schedule.md > /dev/null
cp capacity.json terraform/
```

`terraform apply` then creates real resources in your AWS account and needs an `image_id`.

## Test it

```sh
mise run example:aws
```

The task needs `aws-emulator` on `PATH` and talks to nothing else. It starts the emulator on a free port with its telemetry
off, then checks that:

- both cron formats reproduce the week of every golden output;
- Terraform creates exactly the expected actions, with the right cron, time zone and capacity;
- a second plan is clean, and a changed schedule replaces exactly the old actions;
- the CLI recipe applies a schedule, deletes exactly the old actions and stops when a delete fails.

**What this doesn't prove:** the emulator is not AWS. It doesn't run the actions at their times, so the week is replayed
from the stored crons, and it can't delete EC2 scheduled actions yet, so that part is checked through the plan and the
delete requests. Permissions and quotas stay untested.
