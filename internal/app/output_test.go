// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteOutputCreatesFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "merged.md")

	if err := WriteOutput(out, []byte("content\n"), nil); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	got, err := os.ReadFile(out) //nolint:gosec
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if string(got) != "content\n" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteOutputNoLeftoverTempFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "merged.md")

	if err := WriteOutput(out, []byte("content\n"), nil); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	if len(entries) != 1 || entries[0].Name() != "merged.md" {
		t.Fatalf("expected only merged.md in dir, got %+v", entries)
	}
}

func TestWriteOutputRefusesToOverwriteInput(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")

	if err := os.WriteFile(readme, []byte("original"), 0o644); err != nil { //nolint:gosec
		t.Fatalf("write readme: %v", err)
	}

	err := WriteOutput(readme, []byte("new content"), []string{readme})
	if err == nil {
		t.Fatal("expected error when output path matches an input path")
	}

	got, readErr := os.ReadFile(readme) //nolint:gosec
	if readErr != nil {
		t.Fatalf("read readme: %v", readErr)
	}

	if string(got) != "original" {
		t.Fatalf("input file was modified: %q", got)
	}
}
