// Package files holds the file handling shared by every output: display paths (home shown as "~"), path-free OS
// errors, and atomic writes.
package files

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

// DisplayPath returns path for messages and reports. Relative paths stay as given. Absolute paths inside home are
// shortened to "~" plus the rest, so reports don't spell out the user name. Paths are never resolved to real paths:
// that would make them absolute and leak more, not less.
func DisplayPath(path, home string) string {
	if home == "" || !filepath.IsAbs(path) {
		return path
	}

	rel, err := filepath.Rel(home, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}

	if rel == "." {
		return "~"
	}

	return "~" + string(filepath.Separator) + rel
}

// Reason returns the cause of a file system error without the path it names: *fs.PathError messages repeat the full
// path, which can contain the user name. Callers print their own (display) path next to it.
func Reason(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}

	return err.Error()
}
