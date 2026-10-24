package files

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDisplayPath(t *testing.T) {
	t.Parallel()

	home := filepath.FromSlash("/home/ann")
	sep := string(filepath.Separator)

	tests := []struct {
		path string
		want string
	}{
		{path: "requests/office.md", want: "requests/office.md"},
		{path: "../up/office.md", want: "../up/office.md"},
		{path: filepath.FromSlash("/home/ann/code/office.md"), want: "~" + sep + filepath.FromSlash("code/office.md")},
		{path: home, want: "~"},
		{path: filepath.FromSlash("/home/anna/office.md"), want: filepath.FromSlash("/home/anna/office.md")},
		{path: filepath.FromSlash("/srv/office.md"), want: filepath.FromSlash("/srv/office.md")},
		{path: filepath.FromSlash("/home/office.md"), want: filepath.FromSlash("/home/office.md")},
	}

	for _, tt := range tests {
		if got := DisplayPath(tt.path, home); got != tt.want {
			t.Errorf("DisplayPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}

	if got := DisplayPath(filepath.FromSlash("/home/ann/x.md"), ""); got != filepath.FromSlash("/home/ann/x.md") {
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
