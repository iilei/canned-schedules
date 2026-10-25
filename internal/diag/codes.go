package diag

// Codes documents every diagnostic code, grouped by where it comes from: the document, ranges, META, states,
// configuration, command line, previews, and the tool itself.
var Codes = []CodeDoc{
	{CodeMissingName, "The document has no H1 heading; the H1 names the schedule.", Error},
	{CodeDuplicateName, "More than one H1 heading.", Error},
	{CodeInvalidName, "The name breaks the text rule: 1-80 ASCII letters, digits, spaces or hyphens, starting with a " +
		"letter and ending with a letter or digit.", Error},
	{CodeInvalidSummary, "The summary after the META marker breaks the same text rule as the name.", Error},
	{CodeInvalidMarker, "A directive separates namespace and kind with `:` (`[!SCHEDULE:STATE]`); the separator is " +
		"`-`.", Error},
	{CodeMissingKind, "A marker has the namespace but no kind (`[!SCHEDULE]`, `[!SCHEDULE-]`).", Error},
	{CodeUnknownKind, "A kind other than META, BASE or STATE.", Error},
	{CodeMissingTitle, "A BASE or STATE marker without a state name after it.", Error},
	{CodeDuplicateBlock, "A second META or BASE block.", Error},
	{CodeEmptyState, "A STATE block without ranges.", Error},
	{
		CodeMisplacedDirective,
		"A directive that isn't a top-level quote starting with its marker, e.g. nested in a list " +
			"or wrapped in bold.",
		Error,
	},
	{CodeUnexpectedContent, "Content in a BASE block, which takes only a state name on the marker line.", Error},
	{CodeDisallowedAlert, "An alert that doesn't match `--error-unless-alert-pattern`.", Error},
	{CodeInvalidRange, "A STATE item that isn't `` `start` to `end` WEEKDAYS ``, or content other than a list in a " +
		"STATE block.", Error},
	{CodeInvalidTime, "A time that isn't `H:MM` or `HH:MM` from `00:00` to `23:59` (`24:00` is rejected).", Error},
	{CodeSampleInterval, "A time that isn't a multiple of `--sample-interval`.", Error},
	{CodeInvalidWeekday, "A weekday expression with an unknown or lowercase name, or a broken range or list.", Error},
	{CodeDuplicateWeekday, "The same weekday written twice in one item (`MON,MON`, `WED-WED`).", Error},
	{CodeRepeatedWeekday, "A day that an earlier part of the same item already covers (`MON-WED,TUE`); the range " +
		"still applies.", Warn},
	{CodeMetaSchema, "META content that isn't a list of ``key: `value` `` items, or a key or value the META schema " +
		"rejects.", Error},
	{CodeUnknownState, "A state name the configuration doesn't define, in BASE, STATE or `--base`.", Error},
	{CodeMissingBase, "`--require-base true`, and the document has no BASE block.", Error},
	{CodeConfigSchema, "The configuration, after overrides, fails its JSON Schema.", Error},
	{
		CodeInvalidOverride, "A `--set-config-at` path that doesn't exist, or a `--set-config-value` that isn't JSON.",
		Error,
	},
	{CodeConfigDuplicateState, "Two state ids in the configuration that are equal ignoring case.", Error},
	{
		CodeConfigDuplicateSymbol,
		"Two styles share a symbol, so the `styles` ASCII preset tells those states apart only " +
			"by colour.",
		Warn,
	},
	{CodeStyleReuse, "Fewer styles than states: styles repeat round robin, so states become hard to tell apart.", Warn},
	{CodeInvalidFlag, "An unknown flag or an invalid flag value.", Error},
	{CodeInvalidArgs, "No input file, several files, or flags after the file (exit code 2).", Error},
	{
		CodeInputFile, "The input can't be read: missing, not a regular file, or larger than 256 KiB (exit code 2).",
		Error,
	},
	{CodePreviewApproximate, "Preview buckets mix states, so the preview rounds; reported only when a preview is " +
		"requested.", Warn},
	{CodeOutputSchema, "The tool's own output failed its schema: a bug in canned-schedules, please report it.", Error},
}

// CodeDoc documents one diagnostic code. The README's code table is generated from Codes, and a test keeps Codes
// complete: every Code constant appears exactly once.
type CodeDoc struct {
	Code    string
	Meaning string
	// Severity is the severity the code is reported with.
	Severity Severity
}
