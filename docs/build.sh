#!/bin/sh
# Builds the documentation site into dist/docs (mise run docs:build): Sphinx, Furo and MyST from the hash-pinned
# docs/requirements.txt in a cached virtualenv, and the Sphinx kit and the examples generated with this checkout's
# CLI, the way a team's docs build would use them.
set -eu

venv=${XDG_CACHE_HOME:-$HOME/.cache}/canned-schedules/docs-venv
want=$(sha256sum docs/requirements.txt | cut -d' ' -f1)
if [ "$(cat "$venv/requirements.sha256" 2>/dev/null)" != "$want" ]; then
  rm -rf "$venv"
  python3 -m venv "$venv"
  "$venv/bin/pip" install --quiet --disable-pip-version-check --require-hashes -r docs/requirements.txt
  printf '%s\n' "$want" > "$venv/requirements.sha256"
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
go build -o "$work/canned-schedules" ./cmd/canned-schedules
cs=$work/canned-schedules

# The Sphinx kit, exactly as the playground's download builds it.
rm -rf docs/_ext docs/_generated
"$cs" --sphinx-kit-output-file "$work/kit.zip" .gitlab/issue_templates/canned_schedule_request.md > /dev/null
"$venv/bin/python" -I -c 'import sys, zipfile; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])' "$work/kit.zip" "$work"
cp -R "$work/canned-schedules-sphinx-kit/_ext" docs/_ext

# The examples: every preset the playground lists, as MyST, checked with the preset's own flags.
mkdir -p docs/_generated/examples
"$venv/bin/python" -I -c '
import json, sys
for p in json.load(open("web/presets/index.json")):
    if not p.get("hidden"):
        print(p["id"], "yes" if p.get("args") else "no")
' | while read -r id has_args; do
  src=web/presets/$id.md
  # The office hours preset is the issue template itself (web:build copies it in).
  [ "$id" = office-hours ] && src=.gitlab/issue_templates/canned_schedule_request.md
  set --
  if [ "$has_args" = yes ]; then
    while IFS= read -r line || [ -n "$line" ]; do
      case $line in '' | '#'*) continue ;; esac
      set -- "$@" "$line"
    done < "web/presets/$id.args"
  fi
  "$cs" "$@" --myst-output-file "docs/_generated/examples/$id.md" "$src" > /dev/null
done

rm -rf dist/docs
"$venv/bin/sphinx-build" -W --keep-going -q -b html docs dist/docs
echo "built dist/docs ($(du -sh dist/docs | cut -f1))"
