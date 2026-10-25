#!/bin/sh
# Tests examples/consumer: its mise tasks in a throwaway git repo, with the binary built from this checkout instead of
# the release named in its mise.toml. Run through mise run example:test.
set -eu

root=$(pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

go build -o "$work/canned-schedules" ./cmd/canned-schedules
cp -R examples/consumer "$work/repo"
cd "$work/repo"

export CANNED_SCHEDULES="$work/canned-schedules"
export MISE_TRUSTED_CONFIG_PATHS="$work/repo"
export MISE_DISABLE_TOOLS=github:iilei/canned-schedules
export NO_COLOR=1

git init -q
git config user.email test@example.com
git config user.name test
git config commit.gpgsign false
git add -A
git commit -qm "the example as copied"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}
run() { mise run -q "$@"; }
schedules=schedules/office-hours.canned-schedule

cmp -s "$schedules.md" "$root/.gitlab/issue_templates/canned_schedule_request.md" ||
	fail "the example schedule must be the GitLab issue template"
cmp -s .github/ISSUE_TEMPLATE/canned_schedule_request.md "$root/.github/ISSUE_TEMPLATE/canned_schedule_request.md" ||
	fail "the example's issue template must be the repo's"

# render: three outputs, gitignored, and the text form on stdout.
run schedule:render >"$work/stdout"
for ext in json svg txt; do [ -f "$schedules.$ext" ] || fail "render wrote no .$ext"; done
grep -q '^~~~text$' "$work/stdout" || fail "render must print the text form"
cmp -s "$work/stdout" "$schedules.txt" || fail "printed and written text forms differ"
grep -q '<metadata>' "$schedules.svg" || fail "the SVG must embed the source"
[ -z "$(git status --porcelain -- schedules/*.json schedules/*.svg schedules/*.txt)" ] ||
	fail ".gitignore must cover the outputs"

# A source that fails loses its outputs; outputs without a source go; a tracked output is never deleted.
sed 's/^# Office Hours/# Bad Name!/' "$schedules.md" >schedules/broken.canned-schedule.md
cp "$schedules.json" schedules/broken.canned-schedule.json
if run schedule:render 2>"$work/stderr" >/dev/null; then fail "render must fail for a broken source"; fi
grep -q 'invalid-name' "$work/stderr" || fail "render must report the error"
[ ! -f schedules/broken.canned-schedule.json ] || fail "the failed source's output must go"
rm schedules/broken.canned-schedule.md
touch schedules/gone.canned-schedule.svg schedules/kept.canned-schedule.svg
git add -f schedules/kept.canned-schedule.svg
run schedule:render >/dev/null 2>&1
[ ! -f schedules/gone.canned-schedule.svg ] || fail "an output without a source must go"
[ -f schedules/kept.canned-schedule.svg ] || fail "a tracked output must stay"
git rm -q --cached schedules/kept.canned-schedule.svg
rm schedules/kept.canned-schedule.svg

# check: a valid request (an issue body with CRLF line endings) and one with a namespace typo.
sed 's/$/\r/' "$schedules.md" >"$work/body.md"
run schedule:check "$work/body.md" "$work/ok" || fail "check must pass the issue template"
grep -q '^<!-- canned-schedules -->$' "$work/ok/comment.md" || fail "the comment needs its marker"
grep -q '^~~~~~~~~text$' "$work/ok/comment.md" || fail "the comment must carry the fenced text form"
[ -f "$work/ok/report.json" ] && [ -f "$work/ok/schedule.json" ] || fail "check must write the report and output"
sed 's/\[!NOTE\]/[!SCHEDUEL-STATE]/' "$schedules.md" >"$work/typo.md"
if run schedule:check "$work/typo.md" "$work/bad" 2>/dev/null; then fail "check must fail a namespace typo"; fi
grep -q 'disallowed-alert' "$work/bad/comment.md" || fail "the args file must make alerts strict"
[ ! -f "$work/bad/schedule.txt" ] || fail "an invalid request has no text form"

# pre-commit: the staged content counts, not the working tree; paths with spaces keep their repo path.
mkdir -p "schedules/team a"
sed 's/^# Office Hours/# Bad Name!/' "$schedules.md" >"schedules/team a/on call.canned-schedule.md"
git add -A
cp "$schedules.md" "schedules/team a/on call.canned-schedule.md"
if run schedule:pre-commit 2>"$work/stderr"; then fail "pre-commit must check the staged content"; fi
grep -q '^schedules/team a/on call.canned-schedule.md:1:3: error\[invalid-name\]' "$work/stderr" ||
	fail "pre-commit must name the repo path: $(cat "$work/stderr")"
git add -A
run schedule:pre-commit || fail "pre-commit must pass once the fix is staged"
mise generate git-pre-commit --write --task schedule:pre-commit >/dev/null
git commit -qm "valid" || fail "the hook must let a valid commit through"

echo "examples/consumer: all checks passed"
