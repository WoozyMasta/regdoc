// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"strings"
	"testing"
)

func TestBuildMerge(t *testing.T) {
	sources := []Source{
		{Path: "/root/README.md", RelPath: "README.md", Content: []byte("# Readme\n\nBody.\n")},
		{Path: "/root/CHANGELOG.md", RelPath: "CHANGELOG.md", Content: []byte("# Changelog\n\nEntry.\n")},
	}

	doc, err := Build(sources, []byte("#### Title\n\n---"), BuildConfig{Root: "/root"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	got := string(doc.Content)

	want := "#### Title\n\n---\n\n---\n\n# Readme\n\nBody.\n\n---\n\n# Changelog\n\nEntry.\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}

	if len(doc.Sources) != 2 || doc.Sources[0] != sources[0].Path || doc.Sources[1] != sources[1].Path {
		t.Fatalf("unexpected Sources: %+v", doc.Sources)
	}

	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Fatalf("expected exactly one trailing newline, got %q", got)
	}
}

func TestBuildNoHeader(t *testing.T) {
	sources := []Source{
		{Path: "/root/README.md", RelPath: "README.md", Content: []byte("Body.\n")},
	}

	doc, err := Build(sources, nil, BuildConfig{Root: "/root"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if strings.Contains(string(doc.Content), "---") {
		t.Fatalf("expected no separator with a single part, got %q", doc.Content)
	}
}

func TestBuildSkipsEmptySource(t *testing.T) {
	sources := []Source{
		{Path: "/root/EMPTY.md", RelPath: "EMPTY.md", Content: []byte("")},
		{Path: "/root/README.md", RelPath: "README.md", Content: []byte("Body.\n")},
	}

	doc, err := Build(sources, nil, BuildConfig{Root: "/root"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(doc.Sources) != 1 || doc.Sources[0] != sources[1].Path {
		t.Fatalf("expected only the non-empty source, got %+v", doc.Sources)
	}
}
