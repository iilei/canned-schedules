# timeshapes: Weekly Schedule DSL Plan

## 1. Goal

Build a small, forgiving domain-specific language for describing recurring weekly load profiles. The initial use case is time-based capacity profiles for AWS Auto Scaling Groups, but the core model should remain cloud-provider-independent.

Design principles:
- Easy to author in Markdown issues and pull requests.
- Lenient input syntax, strict and explicit semantics.
- Weekly schedules only; do not implement all of cron.
- Preserve source order because later definitions win conflicts.
- Separate parsing, validation, schedule evaluation, and output rendering.
- Prefer simple behavior that is difficult to use incorrectly.

## 2. Proposed authoring format

Use YAML frontmatter for metadata and tier references, followed by Markdown declarations in the body.

```markdown
---
tiers:
  Tier-A:
    ref: "#/canned-tiers/tier-a"
  Tier-B:
    ref: "#/canned-tiers/tier-b"
  Tier-C:
    ref: "#/canned-tiers/tier-c"
meta:
  not_before: 2026-12-24
---

# Schedule-Name

> [!TIP]
> Definition order matters: later definitions win when transitions conflict.

> [!Tier-A]
> `00:00 FRI`

> [!Tier-B]
> `01:00 SAT`

> [!Tier-A]
> `05:00 SUN`

> [!Tier-C]
> `08:00 MON-FRI`

> [!Tier-A]
> `18:00 MON-FRI`
```

The Markdown body is the human-facing interface. Frontmatter contains structured metadata and references. The parser should recognize only the documented constructs and fail clearly on malformed schedule declarations.

The example uses tier labels as blockquote alert types for readability in supported Markdown renderers. Verify this in the target GitHub/GitLab renderer. If custom alert labels are not portable, adopt a documented fallback such as a bold tier label followed by a code expression.

## 3. Schedule semantics

### 3.1 Transition model

Each declaration means: **switch to this tier at the specified weekly time**.

For example, `Tier-C` at `08:00 MON-FRI` establishes weekday transitions to Tier-C at 08:00. `Tier-A` at `18:00 MON-FRI` establishes transitions back to Tier-A at 18:00.

The active tier carries forward until the next transition in weekly chronological order, including across midnight and the week boundary. This is not a predicate that applies only to minutes matching a cron expression.

### 3.2 Initial state and week boundary

Define the initial state explicitly. Recommended approach: require a transition that establishes the state at the start of the weekly cycle, or provide a required `initial_tier` field in frontmatter. Do not leave the state before the first transition implicit.

The evaluator should treat the schedule as cyclic across the week. A transition late on Sunday can determine the state that carries into Monday.

### 3.3 Ordering and conflicts

- Declarations are processed in source order.
- If multiple declarations resolve to the same weekday and time, the later declaration wins.
- Source order resolves conflicts only; it does not determine how long a tier lasts.
- The duration of a tier is determined by the next transition in weekly chronological order.
- Preserve declaration order explicitly in the parsed model. Never rely on iteration order of a map.

### 3.4 Dates and validity

`meta.not_before` is metadata describing when a schedule may become effective; it does not change the recurring weekly transition pattern. Validate its format and define how downstream consumers use it.

## 4. Time-expression syntax

Accept three equivalent input families where their meaning is unambiguous.

| Form | Example | Meaning |
|---|---|---|
| Friendly | `08:30 MON-FRI` | 08:30 on weekdays |
| Standard five-field cron | `30 8 * * MON-FRI` | 08:30 on weekdays |
| AWS six-field cron | `30 8 ? * MON-FRI *` | 08:30 on weekdays |

These should normalize to the same canonical selector:

```json
{
  "minute": 30,
  "hour": 8,
  "weekdays": ["MON", "TUE", "WED", "THU", "FRI"]
}
```

Tier and source-order information are stored separately from the selector.

### 4.1 Supported subset

The first version should support:
- Minute: integer from 0 through 59.
- Hour: integer from 0 through 23.
- Weekdays: individual weekday names and documented ranges or lists, such as `FRI`, `MON-FRI`, and `SAT,SUN`.
- Friendly `HH:MM` form.
- Five-field Unix-style cron input.
- Six-field AWS-style cron input, subject to the restricted rules below.

Prefer weekday names over numeric weekdays because numeric conventions differ between cron dialects.

### 4.2 Leniency and coercion

The parser may normalize equivalent representations, such as `8:00` and `08:00`, to the same canonical selector. It may accept `*` or `?` in day-of-month and month fields when those fields do not restrict the weekly schedule.

Do not treat `*` and `?` as universally interchangeable cron syntax. Their equivalence is a deliberate rule of this restricted DSL, only in supported fields.

Reject ambiguous or unsupported expressions rather than guessing. In particular, decide and document handling for:
- Seconds or extra fields.
- Numeric weekday values.
- Steps and lists in minute/hour fields.
- Day-of-month restrictions.
- Month restrictions.
- Special AWS cron operators such as `L`, `W`, and `#`.

Recommended first version: reject unsupported constructs and keep the grammar small.

### 4.3 Canonical display

For every accepted declaration, CI should show its normalized interpretation. Example:

```text
Tier-C: 08:00 MON-FRI
      -> minute=0, hour=8, weekdays=MON,TUE,WED,THU,FRI
```

This makes coercion transparent and helps contributors understand validation results.

## 5. Parsing and validation architecture

Recommended pipeline:

1. Extract YAML frontmatter and Markdown body.
2. Parse frontmatter into a typed metadata structure.
3. Walk the Markdown AST and extract supported tier declarations in source order.
4. Parse each time expression using the DSL's explicit dialect detection and grammar.
5. Normalize expressions into strict weekly selectors.
6. Validate tier names, weekday selectors, ranges, metadata, and required initial state.
7. Convert declarations into an ordered list of transitions.
8. Evaluate the weekly schedule.
9. Produce a human-readable weekly preview and canonical machine-readable output.

Important: Markdown AST parsing should identify the supported structural pattern, not infer intent from arbitrary prose. Fail closed when a declaration is malformed or has an unknown tier.

Do not convert declarations to a map before preserving source order.

## 6. Suggested Go model

Illustrative model; refine during implementation.

```go
type TierName string

type Selector struct {
    Minute   int
    Hour     int
    Weekdays []time.Weekday
}

type Transition struct {
    Tier        TierName
    Selector    Selector
    SourceOrder int
    SourceText  string
}

type Schedule struct {
    InitialTier TierName
    Transitions []Transition // ordered by source declaration
}
```

The evaluator can expand each transition to its selected weekly minute positions, sort transitions by weekly position, and build a cyclic sequence of tier intervals. For equal positions, retain source order and select the last declaration.

Keep original source text and source location where possible so errors can point to the exact declaration.

## 7. Canned tiers and metadata

Frontmatter may refer to reusable, versioned tier definitions. The schedule DSL should not embed provider-specific scaling details in each transition.

Responsibilities:
- Tier references resolve to known canned definitions.
- Unknown tier names and unresolved references are errors.
- `meta.not_before` is parsed as a date, not an arbitrary string.
- The effective schedule can be rendered independently of the cloud provider.
- AWS-specific adapters translate the resolved schedule into the required target representation.

Decide whether tier keys are case-sensitive; recommended: exact, documented identifiers, with display labels separate from machine identifiers if needed.

## 8. Validation and feedback

Validation should report:
- The source declaration and its location.
- The reason it is invalid.
- Accepted forms or expected range.
- The normalized form for valid declarations.
- Conflicting transitions and which declaration wins.
- Whether the weekly cycle has a defined initial state.

Example:

```text
line 24: invalid schedule expression "25:72 FUNDAY"
  expected HH:MM followed by weekday selector
  hour must be 0..23
  unknown weekday "FUNDAY"
```

For equal-time conflicts, emit a warning or informational message showing the winning declaration. Keep the initial behavior simple rather than making conflict severity configurable without a concrete need.

## 9. CI and issue feedback

The same parser and evaluator should power local validation and GitHub/GitLab automation.

Initial workflow:
1. A contributor edits the Markdown schedule or submits an issue using the template.
2. CI extracts and validates the schedule.
3. CI posts a comment containing:
   - Validation result.
   - Normalized expressions.
   - Weekly tier visualization.
   - Conflicts and precedence outcomes.
   - Canonical schedule digest.
4. Revisions trigger a fresh preview.

Use one implementation of the parser and evaluator for CLI, CI, and bots so previews cannot disagree with production behavior.

## 10. Canonical representation and digests

Create a stable normalized representation with:
- Schema version.
- Explicit initial tier.
- Ordered declarations, including normalized selector, tier, and source order.
- Resolved weekly transitions or intervals.
- Relevant effective metadata.

Serialize deterministically before hashing. Consider two digests:
- **Request digest:** normalized input declarations and relevant metadata.
- **Effective schedule digest:** canonical resolved weekly tier intervals.

The request digest changes when declarations change, even if the effective schedule remains the same. The effective digest changes only when the resolved schedule changes. Decide which one to use for preview staleness and deployment identity.

## 11. Output and integration

Keep the canonical Go model independent of presentation. Render it into:
- JSON as the canonical machine-readable representation.
- ASCII weekly table for CLI feedback.
- Terraform-friendly JSON if needed.
- Other adapters later, without changing the schedule evaluator.

Make the time-zone policy explicit if downstream behavior depends on local wall-clock time. The initial model should represent recurring wall-clock weekly time, not UTC instants, unless requirements explicitly choose otherwise.

## 12. Testing strategy

### Parser tests
- Equivalent friendly, Unix cron, and AWS cron expressions normalize identically.
- Single weekday, weekday range, and weekday list.
- Leading zeros and accepted flexible formatting.
- Invalid hour/minute values.
- Unknown weekday names.
- Missing, extra, or unsupported fields.
- Unsupported cron operators.
- `*`/`?` coercion only in documented fields.
- Source order is preserved.

### Evaluator tests
- Transitions at midnight.
- Transitions on Sunday and the following Monday.
- Multiple weekdays sharing a transition time.
- Equal-time transitions where the later declaration wins.
- Initial state is always defined.
- Each weekly minute resolves to exactly one tier.
- The final interval wraps correctly across the week boundary.

### Integration tests
- Parse a complete Markdown document.
- Reject malformed frontmatter and unknown tier references.
- Produce stable canonical JSON and digests.
- Verify that CLI and CI previews use identical evaluator results.

Use table-driven Go tests and a small set of golden weekly previews.

## 13. Implementation milestones

### Milestone 1 — Semantics
- Finalize transition semantics.
- Define the initial-state rule.
- Specify exact supported weekday and cron grammar.
- Specify conflict behavior.

**Exit criterion:** examples and edge cases have unambiguous expected results.

### Milestone 2 — Pure parser
- Implement friendly `HH:MM WEEKDAYS` parsing.
- Add five-field cron normalization.
- Add restricted AWS six-field cron normalization.
- Preserve source order and source locations.
- Add table-driven tests.

**Exit criterion:** equivalent forms produce identical selectors; unsupported input fails clearly.

### Milestone 3 — Weekly evaluator
- Convert selectors into weekly transitions.
- Resolve equal-time conflicts by source order.
- Build cyclic tier intervals.
- Add exhaustive or property-style tests ensuring the week is fully covered by exactly one tier.

**Exit criterion:** every weekly time has one deterministic effective tier.

### Milestone 4 — Markdown/frontmatter reader
- Parse YAML frontmatter.
- Extract supported tier declarations from Markdown AST.
- Validate tier references and metadata.
- Produce useful source-aware errors.

**Exit criterion:** the sample schedule parses deterministically and malformed declarations fail closed.

### Milestone 5 — Preview and canonical output
- Implement canonical JSON.
- Implement ASCII weekly preview.
- Add request and effective schedule digests if both prove useful.
- Add golden tests.

**Exit criterion:** local CLI output is stable and understandable.

### Milestone 6 — CI feedback
- Add GitHub/GitLab workflow integration.
- Post validation and preview comments.
- Ensure updates refresh the preview.
- Add a digest/revision marker to identify stale previews.

**Exit criterion:** contributors receive actionable feedback without needing a local toolchain.

## 14. Decisions to resolve before implementation

1. **Initial state:** required `initial_tier` in frontmatter, or require a transition at the start of the weekly cycle?
2. **Week start:** use ISO-style Monday as the canonical first day.
3. **Cron dialect detection:** friendly form by `HH:MM` syntax; otherwise accept only exactly five or six whitespace-separated fields.
4. **Weekday grammar:** define exact case handling and support for ranges/lists.
5. **Time zone:** establish whether the schedule is interpreted in a configured local time zone or simply represents wall-clock time.
6. **Custom alert syntax:** confirm target renderer support for `> [!Tier-A]`; otherwise choose a portable Markdown structure.
7. **Digest purpose:** distinguish source-request identity from effective-schedule identity if both are needed.

## 15. Non-goals for the first version

- Full Unix cron compatibility.
- Full AWS cron compatibility.
- Arbitrary seconds-level schedules.
- General calendar/date exceptions embedded in the weekly selector.
- Parsing schedule meaning from free-form prose.
- Provider-specific behavior inside the core evaluator.
- Silent repair of invalid or ambiguous expressions.

## Definition of done

The first usable version is complete when a contributor can write a Markdown schedule with frontmatter, receive clear validation errors for invalid input, see every accepted expression's canonical meaning, and get a deterministic weekly tier preview. Friendly syntax and supported cron dialects must converge on the same strict internal model, and source-order conflict behavior must be covered by tests.
