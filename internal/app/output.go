// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package app wires target parsing, document discovery/build, provider
// detection, auth resolution and publishing into the regdoc CLI behavior.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// outputFileMode is the Unix mode applied to generated Markdown.
const outputFileMode = 0o644

// WriteOutput atomically writes content to outputPath:
// a temp file is created in the same directory, written, closed, then renamed into place.
// It refuses to overwrite any of inputPaths
// (compared as cleaned absolute paths, without resolving symlinks).
func WriteOutput(outputPath string, content []byte, inputPaths []string) error {
	outAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output path %q: %w", outputPath, err)
	}

	outAbs = filepath.Clean(outAbs)

	for _, in := range inputPaths {
		inAbs, err := filepath.Abs(in)
		if err != nil {
			return fmt.Errorf("resolve input path %q: %w", in, err)
		}

		if sameFilePath(outAbs, filepath.Clean(inAbs)) {
			return fmt.Errorf("output path %q must not overwrite input file %q", outputPath, in)
		}
	}

	return writeAtomic(outAbs, content)
}

// writeAtomic creates the temporary file beside the target
// so rename is atomic on the target filesystem.
func writeAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".regdoc-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file in %q: %w", dir, err)
	}

	tmpPath := tmp.Name()
	ok := false

	defer func() {
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()

	// Persist file data before exposing the replacement name.
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, outputFileMode); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("set permissions on temp file: %w", err)
		}
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Rename publishes the complete temporary file atomically.
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file to %q: %w", path, err)
	}

	if runtime.GOOS != "windows" {
		if err := syncDirectory(dir); err != nil {
			return fmt.Errorf("sync output directory: %w", err)
		}
	}

	ok = true

	return nil
}

// syncDirectory persists a completed rename on Unix filesystems.
func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()

	return dir.Sync()
}

// sameFilePath compares cleaned absolute paths using host case rules.
func sameFilePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}
