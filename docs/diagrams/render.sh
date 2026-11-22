#!/bin/sh
# Renders docs/diagrams/<name>.mmd to <name>-light.svg and <name>-dark.svg next to it (mise run docs:diagrams), with
# mermaid.ink, and records the source's digest in each file, so a test notices a diagram that changed without being
# rendered again. Run it after editing a diagram and commit the SVGs: the docs build itself needs no network. The
# service sees only the diagrams, which are public in this repository anyway.
set -eu

dir=docs/diagrams
for src in "$dir"/*.mmd; do
  name=$(basename "$src" .mmd)
  digest=sha256:$(sha256sum "$src" | cut -d' ' -f1)
  code=$(base64 < "$src" | tr -d '\n' | tr '+/' '-_' | tr -d '=')
  for theme in light dark; do
    mermaid=default
    [ "$theme" = dark ] && mermaid=dark
    out=$dir/$name-$theme.svg
    tmp=$(mktemp)
    curl --fail --silent --show-error --max-time 60 --output "$tmp" "https://mermaid.ink/svg/$code?theme=$mermaid"
    # The digest goes first; the icon font's @import goes (the diagrams use no icons, and an image loads nothing).
    {
      printf '<!-- %s.mmd %s, rendered by mise run docs:diagrams (mermaid.ink, theme %s) -->\n' "$name" "$digest" "$mermaid"
      sed 's#<style xmlns="http://www.w3.org/1999/xhtml">@import url("[^"]*");</style>##' "$tmp"
    } > "$out"
    rm -f "$tmp"
    grep -q '@import' "$out" && { echo "docs:diagrams: $out still imports something" >&2; exit 2; }
  done
  printf 'rendered %s\n' "$name"
done
