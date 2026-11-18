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
| `on-call/main.tf`             | Keeps an SSM parameter up to date with who is on call        |
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

## Who is on call?

`on-call/main.tf` treats each state of a schedule as someone on call, and `id` is the state id. One EventBridge Scheduler schedule per shift start writes the
answer to the SSM parameter `/on-call/current`, with no Lambda or other code to run:

```json
{
  "on_call_current": {"dow": "SUN", "id": "Delphine", "time": "19:00"},
  "on_call_next": {"dow": "MON", "id": "Aldric", "time": "00:00"},
  "on_call_previous": {"dow": "SUN", "id": "Corwin", "time": "14:00"}
}
```

Render the roster next to the file, then apply it. The page's roster needs its own flags, which are in
`web/presets/on-call-roster.args`:

```sh
canned-schedules --json-output-file on-call/on-call.canned-schedule.json schedule.md > /dev/null
terraform -chdir=on-call init
terraform -chdir=on-call apply
```

- **Every shift start is one entry of `occurrences`.** Its `cron.aws` is used unchanged, and the entries before and after it
  are `on_call_previous` and `on_call_next`. Someone who starts at `06:00` from Monday to Friday gets five schedules, each
  writing that day's answer.
- **The week repeats.** A schedule describes one week without dates, so Sunday's last shift hands over to Monday's first, and
  `on_call_previous` during Monday's first shift is Sunday's last.
- **A different roster is a new deployment.** Each apply recalculates every shift and rewrites the parameter for the current
  moment. Right after a change, `on_call_previous` describes the new roster, not the person who actually served.
- **Times are UTC.** Change `schedule_expression_timezone` if your roster is written in another zone.

## Test it

```sh
mise run example:aws
```

The task needs `aws-emulator` on `PATH`. It starts the emulator on a free port with its telemetry off, and **it can't reach a
real AWS account**: it ignores your AWS profiles and credentials, blocks all traffic that isn't local, and stops unless the
endpoint is on this machine and the account is the emulator's `000000000000`. Only `terraform init` goes online, to download
the provider. It then checks that:

- both cron formats reproduce the week of every golden output;
- Terraform creates exactly the expected actions, with the right cron, time zone and capacity;
- a second plan is clean, and a changed schedule replaces exactly the old actions;
- the CLI recipe applies a schedule, deletes exactly the old actions and stops when a delete fails;
- the on-call example gives the right answer for every shift, for the page's 30-person roster and for a rota whose people
  work the same shift on several days.

Each run also writes its Terraform plans as readable JSON to `test/plans/`, which git ignores:

| File                                   | The plan                                            |
| -------------------------------------- | --------------------------------------------------- |
| `1-create-friday-peak.plan.json`       | Creating the service and the first schedule         |
| `2-unchanged.plan.json`                | A second plan, which changes nothing                |
| `3-change-to-night-shift.plan.json`    | Replacing the old actions when the schedule changes |
| `4-create-on-call-roster.plan.json`    | Creating the on-call example for the roster         |
| `5-on-call-roster-unchanged.plan.json` | A second on-call plan, which changes nothing        |

To list what a plan does, use `jq -r '.resource_changes[] | "\(.change.actions | join("/")) \(.address)"' <file>`.

**What this doesn't prove:** the emulator is not AWS. It doesn't run the actions at their times, so the week is replayed
from the stored crons, and it can't delete EC2 scheduled actions yet, so that part is checked through the plan and the
delete requests. Permissions and quotas stay untested.
