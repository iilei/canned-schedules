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
jq -e 'any(.aws_cron.schedules.weekday_maximum[]; .cron == "0 5 ? * MON-FRI *" and .to_tier == "Medium")' "$tmpdir/schedule.json" > /dev/null

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
expect_invalid week-boundary '.capacity_schedules.standard.MON |= sub("^."; "░")'
expect_invalid unknown-tier '.capacity_legend[0].tier = "Ghost"'
expect_invalid duplicate-symbol '.capacity_legend[1].symbol = .capacity_legend[0].symbol'

gomplate -c ".=$tmpdir/schedule.json" -f time-scale.tmpl > "$tmpdir/time-scale.txt"
[ -s "$tmpdir/time-scale.txt" ]
usage_ruler=true usage_legend=true usage_offset=2 gomplate \
	-c ".=$tmpdir/schedule.json" \
	-t time-scale=time-scale.tmpl \
	-f time-table.tmpl > "$tmpdir/time-table-full.txt"
grep -Fq '00:00' "$tmpdir/time-table-full.txt"
grep -Fq 'Legend:' "$tmpdir/time-table-full.txt"
grep -Fq 'MON' "$tmpdir/time-table-full.txt"

gomplate -c ".=$tmpdir/schedule.json" \
	-t time-scale=time-scale.tmpl \
	-t time-table=time-table.tmpl \
	-f time-table-markdown.tmpl > "$tmpdir/time-table.md"
grep -Fq '# Capacity Schedules' "$tmpdir/time-table.md"
grep -Fq '## Configured Schedules' "$tmpdir/time-table.md"
grep -Fq -- '- `all_off`' "$tmpdir/time-table.md"
grep -Fq '## Weekly Schedule Drawings' "$tmpdir/time-table.md"
grep -Fq '~~~text' "$tmpdir/time-table.md"
[ "$(grep -Fc '~~~' "$tmpdir/time-table.md")" -eq 2 ]

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
