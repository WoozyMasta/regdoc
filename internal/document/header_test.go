// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"strings"
	"testing"
)

const mitLicenseText = `MIT License

Copyright (c) 2026 Jane Doe

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`

func TestBuildHeaderGenerated(t *testing.T) {
	dir := t.TempDir()

	got, err := BuildHeader(HeaderConfig{
		Root:       dir,
		Title:      "My Project",
		SourceName: "group/project",
		SourceURL:  "https://git.example/group/project",
		Author:     "Jane Doe",
		Copyright:  "2026 Jane Doe",
	})
	if err != nil {
		t.Fatalf("BuildHeader: %v", err)
	}

	want := "#### My Project\n\n" +
		"Git project: [group/project](https://git.example/group/project)\n\n" +
		"Author: Jane Doe\n\n" +
		"Copyright: 2026 Jane Doe"
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBuildHeaderEmpty(t *testing.T) {
	dir := t.TempDir()

	got, err := BuildHeader(HeaderConfig{Root: dir})
	if err != nil {
		t.Fatalf("BuildHeader: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected empty header, got %q", got)
	}
}

func TestBuildHeaderLicenseAutoDiscover(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "LICENSE", mitLicenseText)

	got, err := BuildHeader(HeaderConfig{Root: dir, Title: "x"})
	if err != nil {
		t.Fatalf("BuildHeader: %v", err)
	}

	if !strings.Contains(string(got), "License: MIT") {
		t.Fatalf("expected License: MIT line, got %q", got)
	}
}

func TestBuildHeaderLicenseExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "COPYING", mitLicenseText)

	got, err := BuildHeader(HeaderConfig{Root: dir, Title: "x", License: path})
	if err != nil {
		t.Fatalf("BuildHeader: %v", err)
	}

	if !strings.Contains(string(got), "License: MIT") {
		t.Fatalf("expected License: MIT line, got %q", got)
	}
}

func TestBuildHeaderLicenseUnrecognizedIsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "LICENSE", "All rights reserved. This is a bespoke, non-standard license text.\n")

	got, err := BuildHeader(HeaderConfig{Root: dir, Title: "x"})
	if err != nil {
		t.Fatalf("BuildHeader: %v", err)
	}

	if strings.Contains(string(got), "License:") {
		t.Fatalf("expected no License line for unrecognized text, got %q", got)
	}
}

func TestBuildHeaderLicenseExplicitMissingIsError(t *testing.T) {
	dir := t.TempDir()

	_, err := BuildHeader(HeaderConfig{Root: dir, Title: "x", License: "does-not-exist.txt"})
	if err == nil {
		t.Fatal("expected error for missing explicit license path")
	}
}

func TestBuildHeaderDoesNotAddDocumentSeparator(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "LICENSE", mitLicenseText)

	header, err := BuildHeader(HeaderConfig{Root: dir, Title: "Project"})
	if err != nil {
		t.Fatalf("BuildHeader: %v", err)
	}

	doc := Merge(header, []ProcessedPart{{Path: "README.md", Content: "# Readme"}})
	if got := strings.Count(string(doc.Content), "---"); got != 1 {
		t.Fatalf("separator count = %d, want 1: %q", got, doc.Content)
	}
}

func TestBuildHeaderUsesDiscoveredMetadataWhenNoExplicitValues(t *testing.T) {
	got, err := BuildHeader(HeaderConfig{
		Root:            t.TempDir(),
		DiscoveredName:  "group/project",
		DiscoveredTitle: "Project",
		DiscoveredURL:   "https://gitlab.example/group/project",
	})
	if err != nil {
		t.Fatalf("BuildHeader: %v", err)
	}

	want := "#### Project\n\nGit project: [group/project](https://gitlab.example/group/project)"
	if string(got) != want {
		t.Fatalf("BuildHeader = %q, want %q", got, want)
	}
}

func TestBuildHeaderExplicitValuesOverrideDiscovered(t *testing.T) {
	got, err := BuildHeader(HeaderConfig{
		Root:            t.TempDir(),
		Title:           "Explicit title",
		SourceName:      "explicit/project",
		SourceURL:       "https://example.com/project",
		DiscoveredTitle: "CI project",
		DiscoveredName:  "ci/project",
		DiscoveredURL:   "https://ci.example/project",
	})
	if err != nil {
		t.Fatalf("BuildHeader: %v", err)
	}

	want := "#### Explicit title\n\nGit project: [explicit/project](https://example.com/project)"
	if string(got) != want {
		t.Fatalf("BuildHeader = %q, want %q", got, want)
	}
}
