#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

cue vet .
cue export . > "$tmpdir/schedule.json"
diff -u test.cue.json "$tmpdir/schedule.json"

cue vet -d '#schedule_contract' schedule-schema.cue test.cue.json
jq '.capacity_schedules.extra = .capacity_schedules.int' test.cue.json > "$tmpdir/valid-extra.json"
cue vet -d '#schedule_contract' schedule-schema.cue "$tmpdir/valid-extra.json"
gomplate -c ".=$tmpdir/valid-extra.json" -t time-scale=time-scale.tmpl -f time-table.tmpl > "$tmpdir/time-table-first.txt"
gomplate -c ".=$tmpdir/valid-extra.json" -t time-scale=time-scale.tmpl -f time-table.tmpl > "$tmpdir/time-table-second.txt"
cmp "$tmpdir/time-table-first.txt" "$tmpdir/time-table-second.txt"
extra_line=$(grep -nF '[extra]' "$tmpdir/time-table-first.txt" | cut -d: -f1)
int_line=$(grep -nF '[int]' "$tmpdir/time-table-first.txt" | cut -d: -f1)
if [ "$extra_line" -ge "$int_line" ]; then
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

expect_invalid missing-day '.capacity_schedules.int |= del(.SUN)'
expect_invalid unknown-day '.capacity_schedules.int.FUNDAY = .capacity_schedules.int.MON'
expect_invalid wrong-length '.capacity_schedules.int.MON = "░"'
expect_invalid unknown-symbol '.capacity_schedules.int.MON |= gsub("▒"; "?")'
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
