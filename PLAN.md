# Plan

The planned update aims to redefine the schedule definition syntax using a set of ordered rules. Each rule assigns a named load profile to a day and/or time range, with later rules overriding earlier rules wherever they overlap. This approach offers a low-barrier entry to define schedules while ensuring unambiguous configuration through CUE validation and transformation.

### Key Changes

1. **Syntax Update**: Define schedules using a set of ordered rules.
2. **Rule Precedence**: Later rules override earlier rules wherever they overlap, with earlier rules continuing to apply elsewhere.
3. **Profile Naming**: Rules reference named load shapes defined by the schedule catalog.
4. **Time Range Notation**: Use `HH:MM-HH:MM` notation for time ranges, which may cross midnight.

### Detailed Plan

1. **Update `schedules.cue`**:
   - Define the new rule-based schedule syntax.
   - Ensure that rules are applied in order to produce a single effective load profile for each point in time.
   - Validate the new syntax using CUE.

2. **Update `README.md`**:
   - Document the new rule-based schedule syntax.
   - Provide examples and explanations for the new syntax.
   - Update the quick start instructions to reflect the new syntax.

3. **Implement Validation**:
   - Ensure that the new syntax adheres to the constraints defined in `schedule-schema.cue`.
   - Use CUE to validate the schedule configuration.

4. **Testing**:
   - Run the `check` task to ensure that the new schedule configuration passes validation.
   - Preview and render schedules to verify that the new syntax is correctly interpreted and rendered.

### Tool Calls

1. **Read `schedules.cue` and `schedule-schema.cue`**:
   - Read the contents of `schedules.cue` and `schedule-schema.cue` to understand the current configuration and validation schema.

2. **Update `schedules.cue`**:
   - Define the new rule-based schedule syntax in `schedules.cue`.
   - Ensure that rules are applied in the correct order.

3. **Run `check` Task**:
   - Use the `check` task to validate the new schedule configuration.

4. **Update `README.md`**:
   - Document the new rule-based schedule syntax in `README.md`.
   - Provide examples and explanations for the new syntax.

5. **Test Rendered Schedules**:
   - Run the `render-time-table` and `render-schedule-svg` tasks to verify that the new syntax is correctly interpreted and rendered.

### Summary

The plan involves updating the schedule definition syntax to a rule-based approach, ensuring unambiguous configuration through CUE validation. This update will be documented in the `README.md`, and the `schedules.cue` file will be modified accordingly. The `check` task will be used to validate the new configuration, and the rendering tasks will be tested to ensure correct interpretation and rendering.

***
Since GitLab still has no native dropdown/radio in issue templates (issue [#353889](https://gitlab.com/gitlab-org/gitlab/-/issues/353889) remains open), you enforce constraints in your **parser + bot** layer:

**1. Define the valid set (CUE or Go):**

```cue
// tiers.cue
package schedules

#Autoscaling: "offline" | "regular" | "medium" | "maximum"
```

or in Go:

```go
var validTiers = map[string]int{
    "offline": 0, "regular": 1, "medium": 2, "maximum": 3,
}
```

**2. Validate at parse time:**

```go
func parseAutoscaling(checked []string) (string, error) {
    if len(checked) == 0 {
        return "", errors.New("no tier selected")
    }
    // "highest wins" — but only among valid tiers
    best := ""
    bestRank := -1
    for _, c := range checked {
        rank, ok := validTiers[c]
        if !ok {
            return "", fmt.Errorf("unknown tier %q (valid: offline, regular, medium, maximum)", c)
        }
        if rank > bestRank {
            best, bestRank = c, rank
        }
    }
    return best, nil
}
```

**3. React via GitLab API (the "constraint" the user actually feels):**

When your pipeline parses the issue and hits an error, it posts a comment:

```go
client.Issues.Comment(projectID, issueIID, gitlab.CreateIssueCommentOptions{
    Body: fmt.Sprintf(
        "⚠️ **Invalid tier:** %q\n\nValid options: `offline`, `regular`, `medium`, `maximum`\n\nPlease edit the issue and check only valid boxes.",
        badValue,
    ),
})
// Also add a label so it's filterable
client.Issues.Update(projectID, issueIID, &gitlab.UpdateIssueOptions{
    Labels: &gitlab.Labels{Add: []string{"needs-attention"}},
})
```

**4. Optionally: block downstream processing**

Your scheduler only picks up issues that have a label like `ready` (which your bot adds *after* successful parse + validation). Invalid issues stay in `needs-attention` and never enter the pipeline.

**Result:**

| User does | What happens |
| ----------- | ------------- |
| Checks `medium` | ✅ Parsed, issue gets `ready` label |
| Checks `medium` + `maximum` | ✅ Highest wins → `maximum` |
| Checks `turbo` | ⚠️ Bot comments, adds `needs-attention` |
| Checks nothing | ⚠️ Bot comments "no tier selected" |
| Adds custom `- [x] Thu–Fri` to Days | ✅ Accepted as free-form (Days are unconstrained) |

The key design choice: **tiers are a closed set** (constrained), **days are an open set** (free-form). Your parser treats them differently — one maps to an enum, the other to `[]string`. The "constraint" is enforced by the bot's comment + label, not by the template itself, since GitLab can't prevent the user from typing anything.

The math is elegant, but it's solving a problem a keyword already solves with less friction:

**The problem with `[00:00-00:00)`:**

- Your users are filing a *request*, not writing a cron expression. They'll see the template and think "do I need brackets or not?"
- You now have to parse and validate bracket syntax, handle the case where they type `[00:00-00:00` (typo), and explain to them *why* the open/closed distinction matters.
- The "easy to use" goal is in tension with this.

**The simpler fix:**

Reserve two keywords in the Hours field:

```markdown
**Hours:**
<!-- e.g. 9:00–17:00, "all-day", or "none" -->
```

| User types | Meaning |
| --- | --- |
| `9:00-17:00` | Normal range |
| `all-day` | 24h, no gaps |
| `none` | No scheduled hours (offline) |

Your parser:

```go
func parseHours(s string) (Hours, error) {
    switch strings.TrimSpace(s) {
    case "all-day":
        return Hours{AllDay: true}, nil
    case "none", "":
        return Hours{None: true}, nil
    default:
        // parse HH:MM-HH:MM
    }
}
```

**When interval notation *would* be worth it:**

- If you need **multiple disjoint ranges** in one field (e.g. `9:00-12:00, 14:00-17:00`), and the 24h case is just the degenerate single-range case. Then `[00:00-00:00)` is the only way to express it without a keyword, and the math is self-consistent.
- If the audience is exclusively engineers who already think in intervals.

For a "file a schedule request" form, the keyword is the right call. Save the interval notation for the internal representation (`Hours{Start, End}` where `Start == End` means 24h, or just a bool), not the user-facing syntax.

-- verdict: does not apply in our case
