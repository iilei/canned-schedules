#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

cue vet .
cue export . > "$tmpdir/schedule.json"
jq -S . test.cue.json > "$tmpdir/expected-schedule.json"
jq -S . "$tmpdir/schedule.json" > "$tmpdir/actual-schedule.json"
diff -u "$tmpdir/expected-schedule.json" "$tmpdir/actual-schedule.json"
jq -e '(.capacity_schedules | keys | map(select(. != "all_off")) | length) == 6' "$tmpdir/schedule.json" > /dev/null
jq -e 'has("_base_schedules") | not' "$tmpdir/schedule.json" > /dev/null
jq -e '.capacity_schedules.all_off | length == 7 and all(.[]; length == 48 and (split("") | all(. == "░")))' "$tmpdir/schedule.json" > /dev/null
jq -e '.aws_cron.timezone == "UTC"' "$tmpdir/schedule.json" > /dev/null
jq -e '.aws_cron.schedules.all_off == []' "$tmpdir/schedule.json" > /dev/null
jq -e 'all(.aws_cron.schedules[]; all(.[]; has("cron")))' "$tmpdir/schedule.json" > /dev/null
jq -e 'any(.aws_cron.schedules.standard[]; .cron == "0 5 ? * MON,TUE,WED,THU,FRI,SAT *" and .to_tier == "Medium")' "$tmpdir/schedule.json" > /dev/null
jq -e 'any(.aws_cron.schedules.standard[]; .cron == "0 0 ? * MON *" and .from_tier == "Off" and .to_tier == "Minimum")' "$tmpdir/schedule.json" > /dev/null
jq -e 'any(.aws_cron.schedules.weekday_maximum[]; .cron == "0 5 ? * MON-FRI *" and .to_tier == "Medium")' "$tmpdir/schedule.json" > /dev/null

jq -r '.capacity_schedules | keys[]' "$tmpdir/schedule.json" > "$tmpdir/schedule-names.txt"
while IFS= read -r schedule; do
	SCHEDULE_NAME="$schedule" gomplate -d "data=$tmpdir/schedule.json" -f schedule.svg.tpl > "$tmpdir/schedule-$schedule.svg"
	[ -s "$tmpdir/schedule-$schedule.svg" ]
	[ "$(grep -Fc '<svg ' "$tmpdir/schedule-$schedule.svg")" -eq 1 ]
done < "$tmpdir/schedule-names.txt"
[ "$(find "$tmpdir" -maxdepth 1 -name 'schedule-*.svg' | wc -l | tr -d ' ')" -eq "$(wc -l < "$tmpdir/schedule-names.txt" | tr -d ' ')" ]
grep -Fq '0 0 ? * MON *' "$tmpdir/schedule-standard.svg"
grep -Fq '<title>cron(0 0 ? * MON *) sets Minimum until the next tier transition</title>' "$tmpdir/schedule-standard.svg"

cue vet -d '#schedule_contract' schedule-schema.cue test.cue.json
jq '.capacity_schedules.extra = .capacity_schedules.standard' test.cue.json > "$tmpdir/valid-extra.json"
cue vet -d '#schedule_contract' schedule-schema.cue "$tmpdir/valid-extra.json"
gomplate -c ".=$tmpdir/valid-extra.json" -t time-scale=time-scale.tmpl -f time-table.tmpl > "$tmpdir/time-table-first.txt"
gomplate -c ".=$tmpdir/valid-extra.json" -t time-scale=time-scale.tmpl -f time-table.tmpl > "$tmpdir/time-table-second.txt"
cmp "$tmpdir/time-table-first.txt" "$tmpdir/time-table-second.txt"
extra_line=$(grep -nF '[extra]' "$tmpdir/time-table-first.txt" | cut -d: -f1)
minimum_line=$(grep -nF '[minimum]' "$tmpdir/time-table-first.txt" | cut -d: -f1)
if [ "$extra_line" -ge "$minimum_line" ]; then
	printf 'Stages are not rendered in deterministic key order.\n' >&2
	exit 1
fi

while IFS= read -r schedule; do
	SCHEDULE_NAME="$schedule" gomplate -c ".=$tmpdir/schedule.json" -t time-scale=time-scale.tmpl -f time-table.tmpl > "$tmpdir/time-table-$schedule.txt"
	[ -s "$tmpdir/time-table-$schedule.txt" ]
	grep -Fq "[$schedule]" "$tmpdir/time-table-$schedule.txt"
done < "$tmpdir/schedule-names.txt"

expect_invalid() {
	case_name=$1
	filter=$2
	jq "$filter" test.cue.json > "$tmpdir/$case_name.json"
	if cue vet -d '#schedule_contract' schedule-schema.cue "$tmpdir/$case_name.json" > "$tmpdir/$case_name.log" 2>&1; then
		printf 'Expected invalid schedule to fail: %s\n' "$case_name" >&2
		exit 1
	fi
}

expect_invalid missing-day '.capacity_schedules.standard |= del(.SUN)'
expect_invalid unknown-day '.capacity_schedules.standard.FUNDAY = .capacity_schedules.standard.MON'
expect_invalid wrong-length '.capacity_schedules.standard.MON = "░"'
expect_invalid unknown-symbol '.capacity_schedules.standard.MON |= gsub("▒"; "?")'
expect_invalid unknown-tier '.capacity_legend[0].tier = "Ghost"'
expect_invalid duplicate-symbol '.capacity_legend[1].symbol = .capacity_legend[0].symbol'

TMPDIR="$tmpdir" sh render.sh table > "$tmpdir/rendered-tables.stdout" 2> "$tmpdir/rendered-tables.stderr"
grep -Fq '[standard]' "$tmpdir/rendered-tables.stdout"
output_dir=$(sed -n 's/^Rendered files: //p' "$tmpdir/rendered-tables.stderr")
[ -s "$output_dir/time-table-standard.txt" ]

TMPDIR="$tmpdir" usage_markdown=true usage_svg=true sh render.sh table > "$tmpdir/rendered-markdown.stdout" 2> "$tmpdir/rendered-markdown.stderr"
grep -Fq '![standard capacity schedule](schedule-standard.svg)' "$tmpdir/rendered-markdown.stdout"
markdown_output_dir=$(sed -n 's/^Rendered files: //p' "$tmpdir/rendered-markdown.stderr")
[ -s "$markdown_output_dir/SCHEDULES.md" ]
[ -s "$markdown_output_dir/schedule-standard.svg" ]

mkdir "$tmpdir/target-dir"
printf 'keep\n' > "$tmpdir/target-dir/unrelated.txt"
printf 'stale\n' > "$tmpdir/target-dir/time-scale.txt"
if TMPDIR="$tmpdir" SCHEDULE_TARGET_DIR="$tmpdir/target-dir" sh render.sh scale > /dev/null 2> "$tmpdir/target-no-force.log"; then
	printf 'Expected target collision to fail without force.\n' >&2
	exit 1
fi
grep -Fq 'keep' "$tmpdir/target-dir/unrelated.txt"
TMPDIR="$tmpdir" SCHEDULE_TARGET_DIR="$tmpdir/target-dir" SCHEDULE_FORCE=true sh render.sh scale > /dev/null 2> /dev/null
grep -Fq '00:00' "$tmpdir/target-dir/time-scale.txt"
grep -Fq 'keep' "$tmpdir/target-dir/unrelated.txt"

gomplate -c ".=$tmpdir/schedule.json" -f time-scale.tmpl > "$tmpdir/time-scale.txt"
[ -s "$tmpdir/time-scale.txt" ]
usage_ruler=true usage_legend=true usage_offset=2 gomplate \
	-c ".=$tmpdir/schedule.json" \
	-t time-scale=time-scale.tmpl \
	-f time-table.tmpl > "$tmpdir/time-table-full.txt"
grep -Fq '00:00' "$tmpdir/time-table-full.txt"
grep -Fq 'Legend:' "$tmpdir/time-table-full.txt"
grep -Fq 'MON' "$tmpdir/time-table-full.txt"
grep -Fq '0 0 ? * MON *' "$tmpdir/time-table-full.txt"

gomplate -c ".=$tmpdir/schedule.json" \
	-t time-scale=time-scale.tmpl \
	-t time-table=time-table.tmpl \
	-f time-table-markdown.tmpl > "$tmpdir/time-table.md"
grep -Fq '# Capacity Schedules' "$tmpdir/time-table.md"
grep -Fq '## Configured Schedules' "$tmpdir/time-table.md"
grep -Fq -- '- `all_off`' "$tmpdir/time-table.md"
grep -Fq '## Weekly Schedule Drawings' "$tmpdir/time-table.md"
while IFS= read -r schedule; do
	grep -Fq "### ${schedule}" "$tmpdir/time-table.md"
	grep -Fq "[$schedule]" "$tmpdir/time-table.md"
done < "$tmpdir/schedule-names.txt"
[ "$(grep -Fc '~~~text' "$tmpdir/time-table.md")" -eq "$(wc -l < "$tmpdir/schedule-names.txt" | tr -d ' ')" ]

usage_svg=true gomplate -c ".=$tmpdir/schedule.json" \
	-t time-scale=time-scale.tmpl \
	-t time-table=time-table.tmpl \
	-f time-table-markdown.tmpl > "$tmpdir/time-table-svg.md"
while IFS= read -r schedule; do
	grep -Fq "![${schedule} capacity schedule](schedule-${schedule}.svg)" "$tmpdir/time-table-svg.md"
done < "$tmpdir/schedule-names.txt"
[ "$(grep -cF '![' "$tmpdir/time-table-svg.md")" -eq "$(wc -l < "$tmpdir/schedule-names.txt" | tr -d ' ')" ]

gomplate -c ".=$tmpdir/schedule.json" -t time-scale=time-scale.tmpl -f time-table.tmpl > "$tmpdir/time-table-default.txt"
if grep -Fq 'Legend:' "$tmpdir/time-table-default.txt" || grep -Fq '00:00' "$tmpdir/time-table-default.txt"; then
	printf 'Default table unexpectedly includes optional ruler or legend.\n' >&2
	exit 1
fi

if env -u usage_offset TIME_SCALE_OFFSET=-1 gomplate -c ".=$tmpdir/schedule.json" -f time-scale.tmpl > /dev/null 2>&1; then
	printf 'Expected a negative time-scale offset to fail.\n' >&2
	exit 1
fi

printf 'Schedule validation, snapshot, and render checks passed.\n'
