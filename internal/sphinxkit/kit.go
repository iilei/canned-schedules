package sphinxkit

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed" // Required for go:embed.
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/iilei/canned-schedules/assets"
	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/myst"
	"github.com/iilei/canned-schedules/templates"
)

// root is the folder inside the zip.
const root = "canned-schedules-sphinx-kit/"

// kitFonts are the fonts the stylesheet uses inside schedule admonitions, and their licence.
var kitFonts = []string{
	"ubuntu-400.woff2", "ubuntu-500.woff2", "ubuntu-mono-400.woff2", "ubuntu-mono-700.woff2",
	"UFL.txt",
}

var (
	//go:embed kit/__init__.py.tmpl
	extensionTemplate string

	//go:embed kit/README.md.tmpl
	readmeTemplate string

	nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

	// modified is the timestamp of every zip entry, so the same input gives the same zip.
	modified = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
)

// Kit builds the Sphinx kit for the request src: the local extension, the stylesheet for Furo, the fonts and the
// request as MyST. version names the generating tool. src must be a valid request.
func Kit(src []byte, cfg *config.Config, version string) ([]byte, error) {
	example, err := myst.Render(src, cfg)
	if err != nil {
		return nil, fmt.Errorf("sphinx kit: %w", err)
	}

	css, err := CSS(cfg, Furo)
	if err != nil {
		return nil, fmt.Errorf("sphinx kit: %w", err)
	}

	digest, err := configDigest(cfg)
	if err != nil {
		return nil, err
	}

	exampleName := slug(firstLine(example)) + ".md"
	data := map[string]string{"version": version, "digest": digest, "example": exampleName}

	files := map[string][]byte{
		"_ext/canned_schedules/static/canned-schedules.css": []byte(css),
		"examples/" + exampleName:                           []byte(example),
		"VERSION":                                           []byte(version + "\nconfig " + digest + "\n"),
	}

	for name, tmpl := range map[string]string{"README.md": readmeTemplate, "_ext/canned_schedules/__init__.py": extensionTemplate} {
		out, err := templates.Execute(tmpl, data)
		if err != nil {
			return nil, fmt.Errorf("sphinx kit: %s: %w", name, err)
		}

		files[name] = out
	}

	for _, f := range kitFonts {
		b, err := fs.ReadFile(assets.Fonts, path.Join("fonts", f))
		if err != nil {
			return nil, fmt.Errorf("sphinx kit: %w", err)
		}

		files["_ext/canned_schedules/static/fonts/"+f] = b
	}

	return zipFiles(files)
}

// zipFiles writes the files under root, sorted and with a fixed timestamp.
func zipFiles(files map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}

	slices.Sort(names)

	var buf bytes.Buffer

	zw := zip.NewWriter(&buf)

	for _, name := range names {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: root + name, Method: zip.Deflate, Modified: modified})
		if err != nil {
			return nil, fmt.Errorf("sphinx kit: %w", err)
		}

		if _, err := w.Write(files[name]); err != nil {
			return nil, fmt.Errorf("sphinx kit: %w", err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("sphinx kit: %w", err)
	}

	return buf.Bytes(), nil
}

// configDigest identifies the effective config (after overrides) the colours come from.
func configDigest(cfg *config.Config) (string, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("sphinx kit: %w", err)
	}

	sum := sha256.Sum256(raw)

	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")

	return strings.TrimPrefix(line, "# ")
}

func slug(name string) string {
	if s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-"); s != "" {
		return s
	}

	return "schedule"
}
