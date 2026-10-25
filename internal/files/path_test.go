package files

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDisplayPath(t *testing.T) {
	t.Parallel()

	// Absolute paths need a volume on Windows ("C:"); elsewhere the volume is empty.
	vol := filepath.VolumeName(t.TempDir())
	abs := func(p string) string { return vol + filepath.FromSlash(p) }
	home := abs("/home/ann")
	sep := string(filepath.Separator)

	tests := []struct {
		path string
		want string
	}{
		{path: "requests/office.md", want: "requests/office.md"},
		{path: "../up/office.md", want: "../up/office.md"},
		{path: abs("/home/ann/code/office.md"), want: "~" + sep + filepath.FromSlash("code/office.md")},
		{path: home, want: "~"},
		{path: abs("/home/anna/office.md"), want: abs("/home/anna/office.md")},
		{path: abs("/srv/office.md"), want: abs("/srv/office.md")},
		{path: abs("/home/office.md"), want: abs("/home/office.md")},
	}

	for _, tt := range tests {
		if got := DisplayPath(tt.path, home); got != tt.want {
			t.Errorf("DisplayPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}

	if got := DisplayPath(abs("/home/ann/x.md"), ""); got != abs("/home/ann/x.md") {
		t.Errorf("without a home directory the path must stay as given, got %q", got)
	}
}

func TestReasonDropsPath(t *testing.T) {
	t.Parallel()

	_, err := os.Open(filepath.FromSlash("/home/ann/secret-user-name/missing.md"))
	if got := Reason(err); got == "" || got == err.Error() || filepath.IsAbs(got) {
		t.Errorf("Reason must drop the path, got %q from %q", got, err)
	}

	if got := Reason(errors.New("plain")); got != "plain" {
		t.Errorf("Reason(plain) = %q", got)
	}
}
