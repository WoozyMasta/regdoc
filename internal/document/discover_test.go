// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeDirEntry is a minimal fs.DirEntry used to test findCaseInsensitive
// without depending on the host filesystem's case sensitivity.
type fakeDirEntry struct{ name string }

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return false }
func (f fakeDirEntry) Type() fs.FileMode          { return 0 }
func (f fakeDirEntry) Info() (fs.FileInfo, error) { return fakeFileInfo{f.name}, nil }

// fakeFileInfo supplies the file metadata required by fakeDirEntry.
type fakeFileInfo struct{ name string }

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

// writeFile creates a test document and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec
		t.Fatalf("write %s: %v", path, err)
	}

	return path
}

func TestDiscoverAuto(t *testing.T) {
	t.Run("readme and changelog in order", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "CHANGELOG.md", "changelog")
		writeFile(t, dir, "README.md", "readme")

		sources, err := Discover(dir, nil)
		if err != nil {
			t.Fatalf("Discover: %v", err)
		}

		if len(sources) != 2 {
			t.Fatalf("expected 2 sources, got %d", len(sources))
		}

		if sources[0].RelPath != "README.md" || sources[1].RelPath != "CHANGELOG.md" {
			t.Fatalf("unexpected order: %+v", sources)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "readme.MD", "readme")

		sources, err := Discover(dir, nil)
		if err != nil {
			t.Fatalf("Discover: %v", err)
		}

		if len(sources) != 1 {
			t.Fatalf("expected 1 source, got %d", len(sources))
		}
	})

	t.Run("no files is not an error", func(t *testing.T) {
		dir := t.TempDir()

		sources, err := Discover(dir, nil)
		if err != nil {
			t.Fatalf("Discover: %v", err)
		}

		if len(sources) != 0 {
			t.Fatalf("expected 0 sources, got %d", len(sources))
		}
	})

}

func TestFindCaseInsensitiveDuplicate(t *testing.T) {
	// Uses synthetic entries because on case-insensitive filesystems (default Windows/macOS),
	// README.md and readme.md cannot coexist as distinct files,
	// making this scenario unreproducible on-disk there.
	entries := []os.DirEntry{
		fakeDirEntry{"README.md"},
		fakeDirEntry{"readme.md"},
	}

	if _, _, err := findCaseInsensitive(entries, "README.md"); err == nil {
		t.Fatal("expected error for duplicate case-variant README files")
	}
}

func TestDiscoverExplicit(t *testing.T) {
	t.Run("preserves order", func(t *testing.T) {
		dir := t.TempDir()
		a := writeFile(t, dir, "a.md", "a")
		b := writeFile(t, dir, "b.md", "b")

		sources, err := Discover(dir, []string{b, a})
		if err != nil {
			t.Fatalf("Discover: %v", err)
		}

		if len(sources) != 2 || sources[0].Path != b || sources[1].Path != a {
			t.Fatalf("unexpected order: %+v", sources)
		}
	})

	t.Run("expands glob pattern in lexical order", func(t *testing.T) {
		dir := t.TempDir()
		docs := filepath.Join(dir, "docs")
		if err := os.Mkdir(docs, 0o755); err != nil {
			t.Fatalf("mkdir docs: %v", err)
		}
		first := writeFile(t, docs, "a.md", "a")
		second := writeFile(t, docs, "b.md", "b")

		sources, err := Discover(dir, []string{filepath.Join(docs, "*.md")})
		if err != nil {
			t.Fatalf("Discover: %v", err)
		}

		if len(sources) != 2 || sources[0].Path != first || sources[1].Path != second {
			t.Fatalf("unexpected sources: %+v", sources)
		}
	})
	t.Run("missing file is an error", func(t *testing.T) {
		dir := t.TempDir()

		if _, err := Discover(dir, []string{filepath.Join(dir, "missing.md")}); err == nil {
			t.Fatal("expected error for missing explicit file")
		}
	})

	t.Run("directory is an error", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "sub")

		if err := os.Mkdir(sub, 0o755); err != nil { //nolint:gosec
			t.Fatalf("mkdir: %v", err)
		}

		if _, err := Discover(dir, []string{sub}); err == nil {
			t.Fatal("expected error for directory explicit path")
		}
	})
}
