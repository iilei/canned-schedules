package cannedschedules

import "github.com/iilei/canned-schedules/internal/schedule"

// FileSuffix ends the name of a schedule file in a catalog (schedules/on-call.canned-schedule.md).
const FileSuffix = schedule.FileSuffix

// FileName is the catalog file name for a schedule's name (its H1): lower case, each run of spaces and hyphens one
// hyphen, plus FileSuffix ("Night Shift" → "night-shift.canned-schedule.md"). The self-hosted playground proposes new
// schedules under it, and catalog checks compare it with a file's actual name. The name rule makes it safe as a file
// name: no slash, no dot, no leading hyphen.
func FileName(name string) string { return schedule.FileName(name) }
