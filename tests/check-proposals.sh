#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

sh proposal.sh demo/a_issue_example_visually.md "$tmpdir/visual"
sh proposal.sh demo/b_issue_example_cue.md "$tmpdir/cue"
jq -e '
	(.capacity_schedules | keys) == ["occassional_load_a"]
	and (.capacity_schedules.occassional_load_a | length == 7 and all(.[]; length == 48))
	and (.aws_cron.schedules | keys) == ["occassional_load_a"]
' "$tmpdir/visual/schedule.json" > /dev/null
jq -e '.capacity_schedules.occassional_load_b.MON | contains("\u2588")' "$tmpdir/cue/schedule.json" > /dev/null

# The generated source must also integrate with the real catalog.
cp schedule-core.cue schedule-schema.cue schedules.cue "$tmpdir/"
cp "$tmpdir/cue/proposal.cue" "$tmpdir/"
cue -C "$tmpdir" vet .
cue -C "$tmpdir" export . -e capacity_schedules > "$tmpdir/combined.json"
jq -e 'has("standard") and has("occassional_load_b")' "$tmpdir/combined.json" > /dev/null

make_proposal() {
	case_name=$1
	filter=$2
	jq "$filter" "$tmpdir/cue/proposal.json" > "$tmpdir/$case_name.json"
	{
		printf '%s\n' '---'
		yq --input-format json --output-format yaml '.' "$tmpdir/$case_name.json"
		printf '%s\n' '---' '# Optional context' 'This text is not schedule input.'
	} > "$tmpdir/$case_name.md"
}

make_proposal custom '.name = "custom" | .schedule_cue |= with_entries(if .value == "_day_maximum" then .value = "(_t1*10) + _t2*28 + _t1*10" else . end)'
sh proposal.sh "$tmpdir/custom.md" "$tmpdir/custom"
jq --arg name occassional_load_a '.capacity_schedules[$name]' "$tmpdir/visual/schedule.json" > "$tmpdir/visual-days.json"
jq '.capacity_schedules.custom' "$tmpdir/custom/schedule.json" > "$tmpdir/custom-days.json"
cmp "$tmpdir/visual-days.json" "$tmpdir/custom-days.json"

make_proposal literal '.name = "literal" | .schedule_cue |= with_entries(.value = "\"\u2591\"*48")'
sh proposal.sh "$tmpdir/literal.md" "$tmpdir/literal"
jq -e '.aws_cron.schedules.literal == [] and (.capacity_schedules.literal | all(.[]; length == 48))' "$tmpdir/literal/schedule.json" > /dev/null

expect_invalid() {
	case_name=$1
	make_proposal "$case_name" "$2"
	if sh proposal.sh "$tmpdir/$case_name.md" > /dev/null 2> "$tmpdir/$case_name.err"; then
		printf 'Expected invalid proposal to fail: %s\n' "$case_name" >&2
		exit 1
	fi
	[ -s "$tmpdir/$case_name.err" ]
}

expect_invalid duplicate '.name = "standard"'
expect_invalid unsafe-name '.name = "../escape"'
expect_invalid unknown-field '.unexpected = true'
expect_invalid both '.schedule_visual = "unused"'
expect_invalid neither 'del(.schedule_cue)'
expect_invalid missing-day 'del(.schedule_cue.SUN)'
expect_invalid unknown-day '.schedule_cue.FUNDAY = "_all_off"'
expect_invalid non-string '.schedule_cue.MON = 48'
expect_invalid unsupported-expression '.schedule_cue.MON = "_all_off } } capacity_tiers: {}"'
expect_invalid malformed-expression '.schedule_cue.MON = "("'
expect_invalid short-day '.schedule_cue.MON = "_t1*47"'
expect_invalid long-day '.schedule_cue.MON = "_t1*49"'
expect_invalid wrong-owner '.owner = "jon_doe"'
expect_invalid short-week 'del(.schedule_cue) | .schedule_visual = "\u2591"'
expect_invalid unknown-symbol 'del(.schedule_cue) | .schedule_visual = ([range(0; 7) | "?" * 48] | join("\n"))'

sed '1d' demo/a_issue_example_visually.md > "$tmpdir/no-frontmatter.md"
if sh proposal.sh "$tmpdir/no-frontmatter.md" > /dev/null 2> "$tmpdir/no-frontmatter.err"; then
	printf 'Expected missing frontmatter to fail.\n' >&2
	exit 1
fi

awk 'NR == 1 || $0 != "---"' demo/a_issue_example_visually.md > "$tmpdir/unclosed.md"
if sh proposal.sh "$tmpdir/unclosed.md" > /dev/null 2> "$tmpdir/unclosed.err"; then
	printf 'Expected unclosed frontmatter to fail.\n' >&2
	exit 1
fi

TMPDIR="$tmpdir" usage_proposal=demo/b_issue_example_cue.md usage_markdown=true usage_ruler=true usage_legend=true usage_offset=2 \
	sh render.sh table > "$tmpdir/preview.md" 2> "$tmpdir/preview.err"
grep -Fq '# Capacity Schedules' "$tmpdir/preview.md"
grep -Fq '### occassional_load_b' "$tmpdir/preview.md"
grep -Fq '00:00' "$tmpdir/preview.md"
grep -Fq 'Legend:' "$tmpdir/preview.md"
grep -Fq 'AWS Cron (UTC):' "$tmpdir/preview.md"
if grep -Fq '### standard' "$tmpdir/preview.md"; then
	printf 'Proposal preview unexpectedly includes the catalog.\n' >&2
	exit 1
fi
output_dir=$(sed -n 's/^Rendered files: //p' "$tmpdir/preview.err")
[ -s "$output_dir/proposal.cue" ]
[ -s "$output_dir/proposal.json" ]
cmp "$tmpdir/preview.md" "$output_dir/SCHEDULES.md"

TMPDIR="$tmpdir" usage_proposal=demo/a_issue_example_visually.md usage_markdown=true usage_svg=true usage_ruler=true usage_offset=2 \
	sh render.sh table > "$tmpdir/svg-preview.md" 2> "$tmpdir/svg-preview.err"
output_dir=$(sed -n 's/^Rendered files: //p' "$tmpdir/svg-preview.err")
grep -Fq '![occassional_load_a capacity schedule](schedule-occassional_load_a.svg)' "$tmpdir/svg-preview.md"
grep -Fq 'class="schedule-ruler"' "$output_dir/schedule-occassional_load_a.svg"

printf 'Proposal validation and preview checks passed.\n'
