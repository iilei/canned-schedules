// Command sitezip packs the built preflight page (dist/site) into the zip that every release attaches, so a team can
// host exactly the page of the version it pins. The zip is deterministic: entries sorted, fixed times and modes, no
// directory entries, index.html at the root.
//
// Usage: go run ./internal/tools/sitezip <site dir> <zip file>
package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const (
	fileMode = 0o644
	dirMode  = 0o750
	// args is the program name plus the site directory and the zip file.
	args      = 3
	exitUsage = 2
)

// modified is every entry's time, as in the Sphinx kit: the zip's bytes depend on the files only.
var modified = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

func main() {
	if len(os.Args) != args {
		fmt.Fprintln(os.Stderr, "usage: sitezip <site dir> <zip file>")
		os.Exit(exitUsage)
	}

	if err := pack(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "sitezip:", err)
		os.Exit(1)
	}
}

func pack(dir, out string) error {
	var names []string

	// The paths are the build's own (mise run site:zip), not user input.
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error { //nolint:gosec // See above.
		if err != nil || d.IsDir() {
			return err
		}

		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: not a regular file", path)
		}

		rel, err := filepath.Rel(dir, path)
		names = append(names, filepath.ToSlash(rel))

		return err
	})
	if err != nil {
		return err
	}

	if !slices.Contains(names, "index.html") {
		return errors.New(dir + " has no index.html: run mise run web:build first")
	}

	slices.Sort(names)

	if err := os.MkdirAll(filepath.Dir(out), dirMode); err != nil { //nolint:gosec // The build's own path.
		return err
	}

	f, err := os.Create(out) //nolint:gosec // The output path is the caller's.
	if err != nil {
		return err
	}

	zw := zip.NewWriter(f)

	for _, name := range names {
		if err := add(zw, filepath.Join(dir, filepath.FromSlash(name)), name); err != nil {
			_ = f.Close()

			return err
		}
	}

	if err := zw.Close(); err != nil {
		_ = f.Close()

		return err
	}

	return f.Close()
}

func add(zw *zip.Writer, path, name string) error {
	h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: modified}
	h.SetMode(fileMode)

	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}

	src, err := os.Open(path) //nolint:gosec // A file of the site directory.
	if err != nil {
		return err
	}

	_, err = io.Copy(w, src)

	return errors.Join(err, src.Close())
}
