// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package document discovers, builds, rewrites and merges
// the Markdown documents published as a repository description.
package document

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Source is a single Markdown document read from disk, before processing.
type Source struct {
	Path    string // Path is the absolute source path.
	RelPath string // RelPath is Path relative to the configured root.
	Content []byte // Content is the unprocessed source Markdown.
}

// Discover resolves the ordered list of Markdown sources to process.
// Explicit paths are expanded as filepath.Glob patterns, in order;
// every resulting path must exist as a regular file.
// Otherwise README.md and CHANGELOG.md are looked up case-insensitively directly under root;
// an empty result (no sources found) is not an error.
func Discover(root string, explicit []string) ([]Source, error) {
	if len(explicit) > 0 {
		return discoverExplicit(root, explicit)
	}
	return discoverAuto(root)
}

// FindOptionalFile searches root (non-recursively, files only) for a file named name, case-insensitively.
// It returns ("", false, nil) if root does not exist or no match is found.
// Multiple case-variant matches (e.g. README.md and readme.md)
// are reported as an error rather than picking one.
func FindOptionalFile(root, name string) (string, bool, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read root %q: %w", root, err)
	}

	match, found, err := findCaseInsensitive(entries, name)
	if err != nil || !found {
		return "", found, err
	}

	return filepath.Join(root, match), true, nil
}

// discoverExplicit loads the requested source paths in order.
func discoverExplicit(root string, paths []string) ([]Source, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root %q: %w", root, err)
	}

	sources := make([]Source, 0, len(paths))

	for _, pattern := range paths {
		matches, err := expandPattern(pattern)
		if err != nil {
			return nil, err
		}

		for _, p := range matches {
			src, err := loadSource(rootAbs, p)
			if err != nil {
				return nil, err
			}

			sources = append(sources, src)
		}
	}

	return sources, nil
}

// expandPattern returns matching paths or the literal pattern when it has no matches.
func expandPattern(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("expand Markdown pattern %q: %w", pattern, err)
	}
	if len(matches) == 0 {
		return []string{pattern}, nil
	}

	return matches, nil
}

// loadSource reads a source and records its path relative to rootAbs.
func loadSource(rootAbs, path string) (Source, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Source{}, fmt.Errorf("resolve path %q: %w", path, err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return Source{}, fmt.Errorf("markdown file %q: %w", path, err)
	}

	if info.IsDir() {
		return Source{}, fmt.Errorf("markdown file %q is a directory, not a file", path)
	}

	content, err := os.ReadFile(abs) //nolint:gosec // path is user-supplied CLI input by design.
	if err != nil {
		return Source{}, fmt.Errorf("read %q: %w", path, err)
	}

	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		return Source{}, fmt.Errorf("resolve %q relative to root %q: %w", path, rootAbs, err)
	}

	return Source{Path: abs, RelPath: rel, Content: content}, nil
}

// discoverAuto loads README.md and CHANGELOG.md when present.
func discoverAuto(root string) ([]Source, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root %q: %w", root, err)
	}

	var sources []Source

	for _, name := range []string{"README.md", "CHANGELOG.md"} {
		path, found, err := FindOptionalFile(root, name)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		src, err := loadSource(rootAbs, path)
		if err != nil {
			return nil, err
		}

		sources = append(sources, src)
	}

	return sources, nil
}

// findCaseInsensitive returns the only case-insensitive file match.
func findCaseInsensitive(entries []os.DirEntry, name string) (string, bool, error) {
	var matches []string

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		if strings.EqualFold(e.Name(), name) {
			matches = append(matches, e.Name())
		}
	}

	switch len(matches) {
	case 0:
		return "", false, nil

	case 1:
		return matches[0], true, nil

	default:
		sort.Strings(matches)
		return "", false, fmt.Errorf(
			"multiple case-variant matches for %q: %s",
			name, strings.Join(matches, ", "),
		)
	}
}
