#!/bin/sh
# Tests examples/aws against a local AWS emulator: the Terraform and CLI recipes with schedules rendered by the binary
# built from this checkout. Run through mise run example:aws. It needs aws-emulator on PATH and proves the recipes against
# the emulator, not against AWS. It refuses to run unless every AWS call can only reach that emulator.
set -eu

root=$(pwd)
image_id=ami-0123456789abcdef0
emulator_account=000000000000
resource=service/imaginary/imaginary-service
# Every Terraform plan of the last run as readable JSON, to proof-read; gitignored.
plans=$root/examples/aws/test/plans

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

command -v aws-emulator >/dev/null || {
	echo "example:aws needs aws-emulator on PATH" >&2
	exit 2
}

work=$(mktemp -d)
emulator_pid=
cleanup() {
	if [ -n "$emulator_pid" ]; then kill "$emulator_pid" 2>/dev/null || true; fi
	rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 1' INT TERM

verify() { python3 "$root/examples/aws/test/verify.py" "$@"; }

# block_outside sends everything but loopback to a port nobody listens on.
block_outside() {
	dead_proxy=http://127.0.0.1:9
	export HTTPS_PROXY=$dead_proxy HTTP_PROXY=$dead_proxy https_proxy=$dead_proxy http_proxy=$dead_proxy
	export NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost
}

# save_plan <tfplan> <name> keeps a plan as $plans/<name>.plan.json.
save_plan() { terraform -chdir="$tf" show -json "$1" | jq . >"$plans/$2.plan.json"; }

rm -rf "$plans"
mkdir -p "$plans"

# The schedules, rendered by this checkout.
go build -o "$work/canned-schedules" ./cmd/canned-schedules
for name in friday-peak night-shift; do
	"$work/canned-schedules" --json-output-file "$work/$name.json" \
		"examples/aws/schedules/$name.canned-schedule.md" >/dev/null
done
capacity=$root/examples/aws/capacity.json

# The on-call example's largest schedule: the page's roster, with its own flags. Every person is a state.
set --
while IFS= read -r line || [ -n "$line" ]; do
	case $line in '' | '#'*) continue ;; esac
	set -- "$@" "$line"
done <web/presets/on-call-roster.args
"$work/canned-schedules" "$@" --json-output-file "$work/on-call-roster.json" web/presets/on-call-roster.md >/dev/null
"$work/canned-schedules" --set-config-at states --set-config-value \
	'[{"id":"Aldric","display-name":"Aldric"},{"id":"Brynja","display-name":"Brynja"},{"id":"Corwin","display-name":"Corwin"}]' \
	--json-output-file "$work/on-call-rota.json" examples/aws/test/on-call-rota.canned-schedule.md >/dev/null

verify golden testdata/golden

# The emulator: on a free port, without telemetry, and with its files in the work directory.
mkdir "$work/emulator"
HOME=$work/emulator aws-emulator --bind 127.0.0.1 --port 0 --telemetry off --auth off --idle-timeout 900 \
	>"$work/emulator.log" 2>&1 &
emulator_pid=$!
endpoint=
for _ in $(seq 1 60); do
	endpoint=$(sed -n 's/.*"endpoint_url":"\(https:[^"]*\)".*/\1/p' "$work/emulator.log" | head -n 1)
	if [ -n "$endpoint" ] && curl -fsk --max-time 2 "$endpoint/__admin/health" >/dev/null; then break; fi
	endpoint=
	sleep 1
done
[ -n "$endpoint" ] || fail "the emulator didn't start: $(tail -n 3 "$work/emulator.log")"

# Nothing from the caller's AWS setup may apply here: no profile, no credentials file, no endpoint of a single service.
for name in $(env | sed -n 's/^\(AWS_[A-Z0-9_]*\)=.*/\1/p'); do unset "$name"; done
: >"$work/aws-config"
: >"$work/aws-credentials"
export AWS_CONFIG_FILE=$work/aws-config AWS_SHARED_CREDENTIALS_FILE=$work/aws-credentials

case $endpoint in
https://127.0.0.1:*) ;;
*) fail "refusing to run: the endpoint $endpoint is not on this machine" ;;
esac

export AWS_ENDPOINT_URL=$endpoint
export AWS_CA_BUNDLE=$work/emulator/.aws/emulator/${endpoint##*:}/ca.pem
export AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=eu-central-1 AWS_DEFAULT_REGION=eu-central-1
export AWS_PAGER='' CHECKPOINT_DISABLE=1 TF_IN_AUTOMATION=1 TF_INPUT=0
export TF_VAR_image_id=$image_id
TF_PLUGIN_CACHE_DIR=${XDG_CACHE_HOME:-$HOME/.cache}/canned-schedules/terraform-plugins
export TF_PLUGIN_CACHE_DIR
mkdir -p "$TF_PLUGIN_CACHE_DIR"

# The guard: outside traffic has to be blocked, and the account has to be the emulator's. Terraform's init still needs the
# registry, so the block starts for good right after it.
blocked=$(block_outside && aws sts get-caller-identity --endpoint-url https://sts.example.invalid \
	--cli-connect-timeout 5 --cli-read-timeout 5 --output text 2>&1 || true)
case $blocked in
*"proxy"*) ;;
*) fail "refusing to run: outside traffic isn't blocked: $blocked" ;;
esac
account=$(block_outside && aws sts get-caller-identity --query Account --output text) ||
	fail "refusing to run: the emulator doesn't answer"
[ "$account" = "$emulator_account" ] || fail "refusing to run: account $account is not the emulator's"
echo "ok: AWS calls can only reach the emulator at $endpoint (account $account)"

# Terraform: the recipes as they are, plus the override that accepts the emulator's credentials.
tf=$work/terraform
cp -R examples/aws/terraform "$tf"
cp examples/aws/test/provider_override.tf "$tf/"
cp "$capacity" "$tf/capacity.json"
cp "$work/friday-peak.json" "$tf/office-hours.canned-schedule.json"
terraform -chdir="$tf" init -no-color >"$work/init.log" || fail "terraform init: $(tail -n 5 "$work/init.log")"

# The on-call example, prepared while init's download is still allowed: the roster, and a rota whose people work the same
# shift on several days.
prepare_on_call() {
	mkdir "$1"
	cp examples/aws/on-call/main.tf examples/aws/test/provider_override.tf "$1/"
	cp "$2" "$1/on-call.canned-schedule.json"
	terraform -chdir="$1" init -no-color >"$work/init.log" || fail "terraform init: $(tail -n 5 "$work/init.log")"
}
prepare_on_call "$work/on-call-roster" "$work/on-call-roster.json"
prepare_on_call "$work/on-call-rota" "$work/on-call-rota.json"

block_outside
terraform -chdir="$tf" plan -no-color -out=create.tfplan >"$work/plan.log" 2>&1 ||
	fail "terraform plan: $(tail -n 8 "$work/plan.log")"
save_plan create.tfplan 1-create-friday-peak
terraform -chdir="$tf" apply -auto-approve -no-color create.tfplan >"$work/apply.log" 2>&1 ||
	fail "terraform apply: $(tail -n 8 "$work/apply.log")"
verify ec2 --group imaginary-service --schedule "$work/friday-peak.json" --capacity "$capacity"
verify ecs --resource "$resource" --schedule "$work/friday-peak.json" --capacity "$capacity"

rc=0
terraform -chdir="$tf" plan -detailed-exitcode -no-color -out=unchanged.tfplan >"$work/plan.log" 2>&1 || rc=$?
[ "$rc" -eq 0 ] || fail "a second plan isn't clean: $(tail -n 12 "$work/plan.log")"
save_plan unchanged.tfplan 2-unchanged
echo "ok: a second plan shows no changes"

# A changed schedule. The emulator can't delete EC2 scheduled actions yet, so the plan is checked for both services
# and only the ECS part is applied.
cp "$work/night-shift.json" "$tf/office-hours.canned-schedule.json"
rc=0
terraform -chdir="$tf" plan -detailed-exitcode -no-color -out=change.tfplan >"$work/plan.log" 2>&1 || rc=$?
[ "$rc" -eq 2 ] || fail "the changed schedule must show up in the plan: $(tail -n 12 "$work/plan.log")"
save_plan change.tfplan 3-change-to-night-shift
verify plan --plan "$plans/3-change-to-night-shift.plan.json" --old "$work/friday-peak.json" --new "$work/night-shift.json"
terraform -chdir="$tf" apply -auto-approve -no-color -target=aws_appautoscaling_scheduled_action.office_hours \
	>"$work/apply.log" 2>&1 || fail "terraform apply: $(tail -n 8 "$work/apply.log")"
verify ecs --resource "$resource" --schedule "$work/night-shift.json" --capacity "$capacity"

# The CLI recipe, on a group of its own.
cli=$work/cli
mkdir "$cli" "$work/bin"
cp examples/aws/cli/apply-schedule.sh "$capacity" "$cli/"
template=$(aws ec2 describe-launch-templates --query 'LaunchTemplates[0].LaunchTemplateId' --output text)
subnet=$(aws ec2 describe-subnets --filters Name=cidr-block,Values=10.42.1.0/24 --query 'Subnets[0].SubnetId' --output text)
aws autoscaling create-auto-scaling-group --auto-scaling-group-name my-asg --min-size 0 --max-size 0 \
	--desired-capacity 0 --vpc-zone-identifier "$subnet" \
	--launch-template "LaunchTemplateId=$template,Version=\$Latest"

recipe() { (cd "$cli" && cp "$work/$1.json" office-hours.canned-schedule.json && bash apply-schedule.sh); }

recipe friday-peak >/dev/null || fail "the CLI recipe must create the first schedule"
verify ec2 --group my-asg --schedule "$work/friday-peak.json" --capacity "$capacity"

# Deletes go through a stand-in: it records them, or fails them on request.
real_aws=$(command -v aws)
cat >"$work/bin/aws" <<STUB
#!/bin/sh
if [ "\$1 \$2" = "autoscaling delete-scheduled-action" ]; then
	[ -z "\${FAIL_DELETES:-}" ] || { echo "stand-in: delete refused" >&2; exit 254; }
	printf '%s\n' "\$*" >>"$work/deletes.log"
	exit 0
fi
exec "$real_aws" "\$@"
STUB
chmod +x "$work/bin/aws"

: >"$work/deletes.log"
PATH=$work/bin:$PATH recipe night-shift >/dev/null || fail "the CLI recipe must apply a changed schedule"
verify deletes --log "$work/deletes.log" --schedule "$work/friday-peak.json"
verify ec2 --group my-asg --schedule "$work/night-shift.json" --capacity "$capacity" --new-only

if FAIL_DELETES=1 PATH=$work/bin:$PATH recipe friday-peak >/dev/null 2>&1; then
	fail "the CLI recipe must stop when a delete fails"
fi
echo "ok: the CLI recipe stops when a delete fails"

# on_call <dir> <schedule> <group> <parameter> [<plan number>] deploys the on-call example and checks its answers.
on_call() {
	tf=$1
	export TF_VAR_schedule_group=$3 TF_VAR_parameter_name=$4
	terraform -chdir="$tf" plan -no-color -out=create.tfplan >"$work/plan.log" 2>&1 ||
		fail "terraform plan (on-call): $(tail -n 12 "$work/plan.log")"
	if [ -n "${5:-}" ]; then save_plan create.tfplan "$5-create-on-call-roster"; fi
	terraform -chdir="$tf" apply -auto-approve -no-color create.tfplan >"$work/apply.log" 2>&1 ||
		fail "terraform apply (on-call): $(tail -n 12 "$work/apply.log")"
	verify oncall --group "$3" --parameter "$4" --schedule "$2"

	rc=0
	terraform -chdir="$tf" plan -detailed-exitcode -no-color -out=unchanged.tfplan >"$work/plan.log" 2>&1 || rc=$?
	[ "$rc" -eq 0 ] || fail "a second on-call plan isn't clean: $(tail -n 12 "$work/plan.log")"
	if [ -n "${5:-}" ]; then save_plan unchanged.tfplan "$(($5 + 1))-on-call-roster-unchanged"; fi
	echo "ok: $3: a second plan shows no changes"

	verify oncall-fire --group "$3" --parameter "$4"
}
on_call "$work/on-call-roster" "$work/on-call-roster.json" on-call /on-call/current 4
on_call "$work/on-call-rota" "$work/on-call-rota.json" on-call-rota /on-call/rota

echo "examples/aws: all checks passed; the Terraform plans are in examples/aws/test/plans"
