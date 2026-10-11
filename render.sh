#!/usr/bin/env sh
set -eu

mode=${1:?usage: render.sh scale|svg|table}
output_dir=$(mktemp -d "${TMPDIR:-/tmp}/canned-schedules.XXXXXX")
schedule_json="$output_dir/schedule.json"

cue export . > "$schedule_json"
gomplate -c ".=$schedule_json" -f time-scale.tmpl > "$output_dir/time-scale.txt"

render_svgs() {
	jq -r '.capacity_schedules | keys[]' "$schedule_json" |
	while IFS= read -r schedule; do
		SCHEDULE_NAME="$schedule" gomplate -d "data=$schedule_json" -f schedule.svg.tpl > "$output_dir/schedule-$schedule.svg"
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
			gomplate -c ".=$schedule_json" \
				-t time-scale=time-scale.tmpl \
				-t time-table=time-table.tmpl \
				-f time-table-markdown.tmpl > "$output_dir/SCHEDULES.md"
		else
			jq -r '.capacity_schedules | keys[]' "$schedule_json" |
			while IFS= read -r schedule; do
				SCHEDULE_NAME="$schedule" gomplate \
					-c ".=$schedule_json" \
					-t time-scale=time-scale.tmpl \
					-f time-table.tmpl > "$output_dir/time-table-$schedule.txt"
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
if [ -n "$target_dir" ]; then
	mkdir -p "$target_dir"
	for artifact in "$output_dir"/*; do
		target="$target_dir/${artifact##*/}"
		if [ -e "$target" ] && [ "${SCHEDULE_FORCE:-false}" != "true" ]; then
			printf 'Target already exists: %s (pass --force to overwrite)\n' "$target" >&2
			exit 1
		fi
	done
	cp -f "$output_dir"/* "$target_dir"/
	artifact_dir=$target_dir
fi

case "$mode" in
	scale)
		cat "$output_dir/time-scale.txt"
		;;
	svg)
		jq -r '.capacity_schedules | keys[]' "$schedule_json" |
		while IFS= read -r schedule; do
			printf '%s\n' "$artifact_dir/schedule-$schedule.svg"
		done
		;;
	table)
		if [ "${usage_markdown:-false}" = "true" ]; then
			cat "$output_dir/SCHEDULES.md"
		else
			jq -r '.capacity_schedules | keys[]' "$schedule_json" |
			while IFS= read -r schedule; do
				cat "$output_dir/time-table-$schedule.txt"
			done
		fi
		;;
esac

printf 'Rendered files: %s\n' "$artifact_dir" >&2
