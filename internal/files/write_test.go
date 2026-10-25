package files

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "report.json")

	if err := os.WriteFile(target, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteFile(target, "report.json", []byte("fresh")); err != nil {
		t.Fatal(err)
	}

	if b, _ := os.ReadFile(target); string(b) != "fresh" {
		t.Errorf("content = %q", b)
	}

	// Windows has no Unix permission bits: files report 0666 or 0444.
	if fi, _ := os.Stat(target); runtime.GOOS != "windows" && fi.Mode().Perm() != fileMode {
		t.Errorf("mode = %v, want %v", fi.Mode().Perm(), os.FileMode(fileMode))
	}

	victim := filepath.Join(dir, "victim.txt")
	link := filepath.Join(dir, "link.json")

	if err := os.WriteFile(victim, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(victim, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	if err := WriteFile(link, "link.json", []byte("report")); err != nil {
		t.Fatal(err)
	}

	if b, _ := os.ReadFile(victim); string(b) != "secret" {
		t.Error("writing must replace a planted symlink, not write through it")
	}

	if err := WriteFile(dir, "dir", []byte("x")); err == nil {
		t.Error("a directory must be refused")
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".canned-schedules-") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}
