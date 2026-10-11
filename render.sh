#!/usr/bin/env sh
set -eu

mode=${1:?usage: render.sh scale|svg|table}
output_dir=$(mktemp -d "${TMPDIR:-/tmp}/canned-schedules.XXXXXX")
schedule_json="$output_dir/schedule.json"
markdown_file="$output_dir/SCHEDULES.md"
svg_filename='schedule'
proposal_dir=''
printf '%s\n' "$output_dir" >&2

if [ -n "${usage_proposal:-}" ]; then
	sh proposal.sh "$usage_proposal" "$output_dir"
	proposal_dir=$(dirname -- "$usage_proposal")
	proposal_stem=$(basename -- "$usage_proposal")
	proposal_stem=${proposal_stem%.*}
	markdown_file="$output_dir/$proposal_stem.generated-text.md"
	svg_filename="$proposal_stem.generated-img"
else
	cue export . > "$schedule_json"
fi
gomplate -c ".=$schedule_json" -f time-scale.tpl > "$output_dir/time-scale.txt"

render_svgs() {
	jq -r '.capacity_schedules | keys[]' "$schedule_json" |
	while IFS= read -r schedule; do
		if [ -n "$proposal_dir" ]; then svg_file="$svg_filename.svg"; else svg_file="schedule-$schedule.svg"; fi
		SCHEDULE_NAME="$schedule" gomplate -d "data=$schedule_json" -f schedule.svg.tpl > "$output_dir/$svg_file"
	done
}

print_svg_manifest() {
	jq -r '.capacity_schedules | keys[]' "$schedule_json" |
	while IFS= read -r schedule; do
		printf '%s\n' "$output_dir/schedule-$schedule.svg"
	done
}

case "$mode" in
	scale)
		;;
	svg)
		render_svgs
		;;
	table)
		if [ "${usage_markdown:-false}" = "true" ]; then
			if [ "${usage_svg:-false}" = "true" ]; then
				render_svgs
			fi
			# Proposal SVGs are optional sidecars; issue Markdown stays text.
			markdown_svg=${usage_svg:-false}
			if [ -n "$proposal_dir" ]; then markdown_svg=false; fi
			usage_svg="$markdown_svg" gomplate -c ".=$schedule_json" \
				-t time-scale=time-scale.tpl \
				-t time-table=time-table.tpl \
				-f time-table-markdown.tpl > "$markdown_file"
		else
			jq -r '.capacity_schedules | keys[]' "$schedule_json" |
			while IFS= read -r schedule; do
				SCHEDULE_NAME="$schedule" gomplate \
					-c ".=$schedule_json" \
					-t time-scale=time-scale.tpl \
					-f time-table.tpl > "$output_dir/time-table-$schedule.txt"
			done
		fi
		;;
	*)
		printf 'Unknown render mode: %s\n' "$mode" >&2
		exit 2
		;;
esac

artifact_dir=$output_dir
target_dir=${SCHEDULE_TARGET_DIR:-}
set -- "$output_dir"/*
if [ -z "$target_dir" ] && [ -n "$proposal_dir" ]; then
	if [ "$mode" = "table" ] && [ "${usage_markdown:-false}" = "true" ]; then
		target_dir=$proposal_dir
		set -- "$markdown_file" "$output_dir/$proposal_stem.generated.patch"
		if [ "${usage_svg:-false}" = "true" ]; then set -- "$@" "$output_dir/$svg_filename.svg"; fi
	elif [ "$mode" = "svg" ]; then
		target_dir=$proposal_dir
		set -- "$output_dir/$svg_filename.svg" "$output_dir/$proposal_stem.generated.patch"
	fi
fi
if [ -n "$target_dir" ]; then
	mkdir -p "$target_dir"
	for artifact do
		target="$target_dir/${artifact##*/}"
		if [ -e "$target" ] && [ "${SCHEDULE_FORCE:-false}" != "true" ]; then
			printf 'Target already exists: %s (pass --force to overwrite)\n' "$target" >&2
			exit 1
		fi
	done
	cp -f "$@" "$target_dir"/
	artifact_dir=$target_dir
fi

case "$mode" in
	scale)
		cat "$output_dir/time-scale.txt"
		;;
	svg)
		jq -r '.capacity_schedules | keys[]' "$schedule_json" |
		while IFS= read -r schedule; do
			if [ -n "$proposal_dir" ]; then svg_file="$svg_filename.svg"; else svg_file="schedule-$schedule.svg"; fi
			printf '%s\n' "$artifact_dir/$svg_file"
		done
		;;
	table)
		if [ "${usage_markdown:-false}" = "true" ]; then
			cat "$markdown_file"
		else
			jq -r '.capacity_schedules | keys[]' "$schedule_json" |
			while IFS= read -r schedule; do
				cat "$output_dir/time-table-$schedule.txt"
			done
		fi
		;;
esac

printf 'Rendered files: %s\n' "$artifact_dir" >&2
