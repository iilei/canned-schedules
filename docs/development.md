# Development

```sh
mise run check      # format check, vet, lint, tests (race detector), build
mise run test:wasm  # the same tests compiled to WebAssembly, run under Node
mise run web:serve  # build and serve the preflight page
mise run fmt        # apply formatting
mise run api:diff   # the Go library's API changes since the last release
```

Releases come from a signed tag (`git tag -s vX.Y.Z -m vX.Y.Z`, then push it): the release workflow builds a draft,
smoke-tests every binary and only then publishes it. Don't create releases on GitHub's Releases page; the workflow
refuses a tag whose release is already published.

The spec lives in [`testdata/golden`](https://github.com/iilei/canned-schedules/tree/master/testdata/golden): one directory per case with the input, optional CLI
arguments, and the expected output or diagnostics. `python3 testdata/golden/gen.py` regenerates them from an
independent reference implementation; review the diff like any other change. Never derive fixtures from the Go code.

The page's editor is [CodeMirror 6](https://codemirror.net/), bundled from [`editor/`](https://github.com/iilei/canned-schedules/tree/master/editor) by `web:build` with the
versions locked in `editor/package-lock.json` (`npm ci`, install scripts off). The licences of the bundled packages
are published next to it as `editor-licenses.txt`. Without `editor.js` the page falls back to a plain textarea.
