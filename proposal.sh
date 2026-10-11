#!/usr/bin/env sh
set -eu

proposal=${1:?usage: proposal.sh proposal.md [output-dir]}
output_dir=${2:-}
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/canned-proposal.XXXXXX")
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

awk '
	{sub(/\r$/, "")}
	NR == 1 {
		if ($0 != "---") {
			print "Proposal must start with YAML frontmatter (---)." > "/dev/stderr"
			exit 1
		}
		next
	}
	$0 == "---" {closed = 1; exit}
	END {
		if (!closed) {
			print "Proposal must have a closing frontmatter delimiter (---)." > "/dev/stderr"
			exit 1
		}
	}
' "$proposal"
yq --front-matter extract --input-format yaml --output-format json '.' "$proposal" > "$tmpdir/proposal.json"

jq -e '
	if type != "object" then error("Frontmatter must be an object") else . end
	| if (keys - ["name", "description", "team", "owner", "schedule_visual", "schedule_cue"] | length) != 0
	  then error("Unknown proposal fields") else . end
	| if (.name | type != "string") then error("Schedule name must be a string") else . end
	| if (.name | test("^[a-z][a-z0-9_]*$") | not)
	  then error("Schedule name must use lowercase letters, digits, and underscores, starting with a letter") else . end
	| if (.description != null and (.description | type != "string"))
	  then error("Description must be a string or null") else . end
	| if (.team != null and (.team | type != "string"))
	  then error("Team must be a string or null") else . end
	| if has("owner") and ((.owner | type != "array") or (.owner | any(.[]; type != "string")))
	  then error("Owner must be a list of strings") else . end
	| if (has("schedule_visual") == has("schedule_cue"))
	  then error("Provide exactly one of schedule_visual and schedule_cue") else . end
	| if has("schedule_visual") then
		if (.schedule_visual | type != "string") then error("schedule_visual must be a string") else . end
		| (.schedule_visual | sub("\n$"; "") | split("\n")) as $rows
		| if ($rows | length) != 7 then error("schedule_visual must contain seven rows, ordered MON-SUN") else . end
	else
		if (.schedule_cue | type != "object") then error("schedule_cue must be a weekday map") else . end
		| if (.schedule_cue | keys) != (["MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"] | sort)
		  then error("schedule_cue must contain exactly MON-SUN") else . end
		| if (.schedule_cue | any(.[]; type != "string"))
		  then error("schedule_cue values must be CUE expression strings") else . end
		| if (.schedule_cue | any(.[]; test("^([ \t]|_day_maximum|_day_medium|_day_minimum|_all_off|_t[0-3]|[0-9]+|[+*()]|\"[\u2591\u2592\u2593\u2588]*\")+$") | not))
		  then error("CUE expressions may use day helpers, _t0-_t3, slot string literals, integers, +, *, and parentheses") else . end
	end
' "$tmpdir/proposal.json" > /dev/null

name=$(jq -r '.name' "$tmpdir/proposal.json")
cd "$repo_root"
cue export . -e capacity_schedules > "$tmpdir/catalog.json"
if jq -e --arg name "$name" 'has($name)' "$tmpdir/catalog.json" > /dev/null; then
	printf 'Schedule already exists in the catalog: %s\n' "$name" >&2
	exit 1
fi

mkdir "$tmpdir/model"
cp schedule-core.cue schedule-schema.cue "$tmpdir/model/"
jq -r '
	["MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"] as $days
	| (if has("schedule_visual") then
		(.schedule_visual | sub("\n$"; "") | split("\n")) as $rows
		| [range(0; 7) | {key: $days[.], value: ($rows[.] | tojson)}] | from_entries
	   else .schedule_cue end) as $schedule
	| "package schedules\n\n_base_schedules: {\n  \(.name | tojson): {",
	  ($days[] as $day | "    \($day): \($schedule[$day])"),
	  "  }\n}"
' "$tmpdir/proposal.json" > "$tmpdir/model/proposal.cue"
printf 'package schedules\n\ncapacity_schedules: _base_schedules\n' > "$tmpdir/model/catalog.cue"
cue fmt "$tmpdir/model/proposal.cue"
cue -C "$tmpdir/model" vet .
cue -C "$tmpdir/model" export . > "$tmpdir/schedule.json"
cue vet -d '#schedule_contract' schedule-schema.cue "$tmpdir/schedule.json"

if [ -n "$output_dir" ]; then
	mkdir -p "$output_dir"
	cp "$tmpdir/proposal.json" "$tmpdir/model/proposal.cue" "$tmpdir/schedule.json" "$output_dir/"
fi
printf 'Validated proposal: %s\n' "$name" >&2
