# Usage

Prebuilt binaries for Linux (x86_64, arm64, armv7), macOS (universal) and Windows (amd64, arm64) are attached to
each [GitHub release](https://github.com/iilei/canned-schedules/releases). Every archive comes with a GPG signature
(`.sig`), a SHA-256 sidecar (`.sha256`) and an SPDX SBOM (`.sbom.json`); `checksums.txt` lists them all and is signed
as well.

- **The release key** has the fingerprint `B645 234C 9CB8 175E CA5D  78A5 C844 7A22 52E9 E7B6` (ed25519). Its public
  half is [`release-key.asc`](https://github.com/iilei/canned-schedules/blob/master/release-key.asc), attached to every release and published on
  [GitHub](https://github.com/iilei.gpg), keys.openpgp.org and keyserver.ubuntu.com. In a checkout,
  `mise run release:verify <dir>` checks a downloaded release (`gh release download <tag> --dir <dir>`) against that
  key alone, plus the checksums and sidecars.
- **Build provenance:** every file in `checksums.txt` has a GitHub attestation that it was built by this repo's
  release workflow: `gh attestation verify <archive> --repo iilei/canned-schedules` (gh 2.49 or newer; Ubuntu's
  packaged gh may be older).
- **With [mise](https://mise.jdx.dev),** `mise use github:iilei/canned-schedules@<version>` installs a release binary
  and checks it against the SHA-256 digest GitHub records for the download and against its provenance attestation
  (not the GPG signature).

Or build from a checkout (Go 1.26 or newer, tools pinned via [mise](https://mise.jdx.dev)):

```sh
mise install
go build ./cmd/canned-schedules
./canned-schedules [flags] schedule.md
```

The input is always a regular file of at most 256 KiB, given after the flags. On success the canonical JSON goes to
stdout and the exit code is 0. Otherwise each problem goes to stderr as `path:line:col`, with exit code 1 for errors in
the document or in flag values and 2 for an unusable command line or environment (no file, several files, flags after
the file, unreadable input, report not writable):

```text
schedule.md:9:17: error[invalid-time]: 24:00 is not a valid time: the end of a range is exclusive, so a range that runs until midnight ends at 00:00, e.g. `21:45` to `00:00`
```

`canned-schedules --help` prints the same list.

<!-- generated: flags (mise run docs; from the flag set in internal/cli) -->
**Evaluation**

| Flag                                   | Default     | Meaning                                                                                             |
| -------------------------------------- | ----------- | --------------------------------------------------------------------------------------------------- |
| `--sample-interval <minutes>`          | `1`         | Every time must be a multiple of it; must divide 60 (e.g. `15`, `60`)                               |
| `--require-base true\|false`           | `false`     | `true`: the document must contain a BASE block; can't be combined with `--base`                     |
| `--base <state>`                       | first state | Base state when the document has no BASE block                                                      |
| `--error-unless-alert-pattern <regex>` |             | Every alert other than a schedule directive must match (whole type, Go syntax, `(?i)` for any case) |

**Configuration**

| Flag                        | Default | Meaning                                                                                                                     |
| --------------------------- | ------- | --------------------------------------------------------------------------------------------------------------------------- |
| `--set-config-at <path>`    |         | Config path to replace, e.g. `representation.styles.states[0]` (repeatable)                                                 |
| `--set-config-value <json>` |         | JSON value for the matching `--set-config-at` (repeatable)                                                                  |
| `--ascii-symbols <preset>`  | `ascii` | Symbols in the ASCII preview: `ascii`, `braille` or `styles` (shorthand for `--set-config-at representation.ascii.symbols`) |

**Outputs**

| Flag                              | Default | Meaning                                                                                                                                                                                                 |
| --------------------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--output json\|ascii\|text`      | `json`  | What stdout carries: the canonical JSON, the ASCII preview or the text form (the schedule in canonical form with the ASCII preview)                                                                     |
| `--json-output-file <path>`       |         | Write the canonical JSON (only when the document has no errors)                                                                                                                                         |
| `--svg-output-file <path>`        |         | Write an SVG preview (only when the document has no errors)                                                                                                                                             |
| `--svg-theme auto\|light\|dark`   | `auto`  | SVG colours: `auto` (follow the viewer), `light` or `dark`                                                                                                                                              |
| `--svg-embed-source`              |         | Embed the schedule in canonical form in the SVG's metadata; needs `--svg-output-file`                                                                                                                   |
| `--svg-annotations`               |         | Show a META section above the transition table: the summary and the annotations (`x-*` keys); needs `--svg-output-file`                                                                                 |
| `--ascii-output-file <path>`      |         | Write an ASCII preview (only when the document has no errors)                                                                                                                                           |
| `--text-output-file <path>`       |         | Write the text form: the schedule in canonical form with the ASCII preview, itself a valid schedule (only when the document has no errors)                                                              |
| `--color auto\|always\|never`     | `auto`  | Colours in the ASCII preview; `auto` means on a terminal only, and not with `NO_COLOR`                                                                                                                  |
| `--preview-downsample[=N]`        | `15`    | Preview buckets in minutes (must divide 60); `=N` needs the `=`                                                                                                                                         |
| `--no-preview-downsample`         |         | Preview at 1-minute resolution (1440 columns per day)                                                                                                                                                   |
| `--myst-output-file <path>`       |         | Write the schedule as MyST for Sphinx (only when the document has no errors)                                                                                                                            |
| `--sphinx-kit-output-file <path>` |         | Write the Sphinx kit zip (only when the document has no errors)                                                                                                                                         |
| `--trace-output-file <path>`      |         | Write the paint trace as JSON: what each range covers, where it shows and where later ranges paint over it; a debugging aid, its shape may change in any release (only when the document has no errors) |

**Error report**

| Flag                              | Default | Meaning                                                                   |
| --------------------------------- | ------- | ------------------------------------------------------------------------- |
| `--error-output-file <path>`      | stderr  | Write the error report to this file (always, even when empty)             |
| `--error-output-format txt\|json` | `txt`   | Error report format: `txt` or `json`                                      |
| `--error-report-template <name>`  |         | Render the error report with the built-in `markdown-gfm` template instead |

**Other**

| Flag        | Default | Meaning                                                                                                                         |
| ----------- | ------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `--verbose` |         | Print the paint trace to stderr, one `file:line:col` line per range and one for the base (only when the document has no errors) |
| `--help`    |         | Print this help and exit                                                                                                        |
| `--version` |         | Print the version and exit                                                                                                      |
<!-- end generated -->
