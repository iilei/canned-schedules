# Stability

Until 1.0, any release may break things; a breaking change always comes with a new `schema-version` and a new
minor version of the tool (`0.1.x` → `0.2.0`). Compatible changes only raise the patch version (`0.1.0` → `0.1.1`),
so a range such as `^0.1` or `~0.1` never picks up a breaking change.

- **`schema-version` is a date:** the date of the last breaking change to any of the contracts, which are the document
  syntax with its META, the configuration, the JSON output and the error report. This version reads and writes
  `2026-11-18`. Documents don't have to declare it; one that declares another version is rejected with a hint.
- **The schemas** of each version are published at
  `https://iilei.github.io/canned-schedules/schema/<date>/` (`output`, `report`, `config` and `meta`, each
  `.schema.json`) and attached to every release, listed in its signed `checksums.txt`. The tool never fetches them;
  they're built in.
- **Not breaking (the date stays):** new fields in the output or the report, new warning codes, new flags, accepting
  input that used to be rejected, clearer descriptions. The output and report schemas allow additional properties,
  so newer output still validates against the schema of its date. Read the output tolerantly: ignore fields and
  warning codes you don't know.
- **Breaking (a new date):** removing or renaming a field, changing its type, making a field required, tightening a
  pattern, rejecting input that used to be accepted, and changing what a field
  means, which no schema can detect. A change of meaning comes as a new field instead; the old one goes at the next
  breaking change.
- **History** (each date, what broke, the first release with it):
  - `2026-11-18` (`v0.3.0`): `constraints` is required in the output. Documents that declare an earlier
    `schema-version` have to update it or drop the line.
  - `2026-10-26` (`v0.2.0`): `digests` removed from the output; compare or hash `base` and `week` yourself.
  - `2026-10-25` (`v0.0.1-pre.6`): the first date, replacing `schema-version` `1`; the output and report schemas
    allow additional properties.
- **The Go library's API** (the package at the module root) follows the tool's version: before 1.0 a breaking API
  change raises the minor version, a compatible one the patch. Its data types mirror the output schema of the
  `schema-version` they carry.
- **Not part of the contract:** diagnostic messages (their codes are), the layout of the previews, the
  [paint trace](previews.md#paint-trace) (`--verbose` and `--trace-output-file`, although its schema is published too), and the
  JSON's key order (stable, but parse the JSON rather than matching its text).
