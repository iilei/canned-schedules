package report

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const fileMode = 0o644

// WriteFile writes data to path atomically: it writes a temporary file in the same directory and renames it into
// place. Readers never see a half-written report, and a symlink planted at path is replaced rather than written
// through. A directory at path is refused. Errors name the file by display, never by path.
func WriteFile(path, display string, data []byte) error {
	fail := func(err error) error {
		return fmt.Errorf("--error-output-file %s: %s", display, Reason(err))
	}

	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return fmt.Errorf("--error-output-file %s is a directory", display)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".canned-schedules-report-*")
	if err != nil {
		return fail(err)
	}

	if err := fill(tmp, data); err != nil {
		return errors.Join(fail(err), os.Remove(tmp.Name()))
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return errors.Join(fail(err), os.Remove(tmp.Name()))
	}

	return nil
}

// fill writes data, sets the file mode and closes f.
func fill(f *os.File, data []byte) error {
	if _, err := f.Write(data); err != nil {
		return errors.Join(err, f.Close())
	}

	if err := f.Chmod(fileMode); err != nil {
		return errors.Join(err, f.Close())
	}

	return f.Close()
}
