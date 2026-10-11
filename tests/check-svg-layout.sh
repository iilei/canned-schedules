#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

cue export . > "$tmpdir/schedule.json"
jq '.aws_cron.schedules.standard = []' "$tmpdir/schedule.json" > "$tmpdir/empty.json"
jq '.aws_cron.schedules.standard |= [.[0] | .from_tier = "Off" | .to_tier = "Off"]' \
	"$tmpdir/schedule.json" > "$tmpdir/short.json"
jq '.aws_cron.schedules.standard |= [.[0] |
	.from_tier = "An unusually long source tier" |
	.to_tier = "An unusually long destination tier" |
	.cron = "0 0 ? * MON,TUE,WED,THU,FRI,SAT,SUN *"]' \
	"$tmpdir/schedule.json" > "$tmpdir/long.json"

for data in "$tmpdir/schedule.json" "$tmpdir/empty.json" "$tmpdir/short.json" "$tmpdir/long.json"; do
	transition_x=$(jq '40 + (.time_brackets | length) * 12 + 28 + 6' "$data")
	transition_w=$(jq '
		.aws_cron.schedules.standard |
		if length == 0 then ("No tier transitions" | length) * 8
		else [("Transition" | length) * 9, (.[] | (.from_tier + " -> " + .to_tier | length) * 8)] | max end
	' "$data")
	cron_x=$((transition_x + transition_w + 18))
	cron_w=$(jq '[("AWS Cron Expression" | length) * 9,
		(.aws_cron.schedules.standard[] | (.cron | length) * 8)] | max' "$data")
	total_w=$((cron_x + cron_w + 12))
	for ruler in false true; do
		SCHEDULE_NAME=standard usage_ruler="$ruler" usage_offset=2 \
			gomplate -d "data=$data" -f schedule.svg.tpl > "$tmpdir/render.svg"
		grep -Eq "x=\"$transition_x\" .*font-size=\"14\".*>Transition</text>" "$tmpdir/render.svg"
		grep -Eq "x=\"$cron_x\" .*font-size=\"14\".*>AWS Cron Expression</text>" "$tmpdir/render.svg"
		grep -Fq "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"$total_w\"" "$tmpdir/render.svg"
		grep -Eq "x=\"$transition_x\" y=\"47\" font-size=\"18\".*>standard</text>" "$tmpdir/render.svg"
		if [ "$ruler" = true ]; then
			grep -Fq 'height="342"' "$tmpdir/render.svg"
			grep -Eq 'x="40" dx="-2ch" y="47".*>00:00</text>' "$tmpdir/render.svg"
		else
			grep -Fq 'height="322"' "$tmpdir/render.svg"
		fi
		transition_line=$(grep -n '>Transition</text>' "$tmpdir/render.svg" | cut -d: -f1)
		cron_line=$(grep -n '>AWS Cron Expression</text>' "$tmpdir/render.svg" | cut -d: -f1)
		[ "$transition_line" -lt "$cron_line" ]
		if [ "$data" = "$tmpdir/empty.json" ]; then
			grep -Eq "x=\"$transition_x\" .*font-size=\"12\".*>No tier transitions</text>" "$tmpdir/render.svg"
		else
			grep -Eq "x=\"$transition_x\" .*font-size=\"12\".* -&gt; .*</text>" "$tmpdir/render.svg"
			grep -Eq "x=\"$cron_x\" .*font-size=\"12\".*>0 " "$tmpdir/render.svg"
		fi
	done
done

printf 'SVG transition-first layout and content-derived widths passed.\n'
