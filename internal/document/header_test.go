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
	clearProjectEnv(t)

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

func TestBuildHeaderDiscoversCIProject(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "GitLab",
			env: map[string]string{
				"CI_PROJECT_PATH":  "group/project",
				"CI_PROJECT_TITLE": "Project",
				"CI_PROJECT_URL":   "https://gitlab.example/group/project",
			},
			want: "#### Project\n\nGit project: [group/project](https://gitlab.example/group/project)",
		},
		{
			name: "GitHub compatible Actions",
			env: map[string]string{
				"GITHUB_SERVER_URL": "https://forge.example",
				"GITHUB_REPOSITORY": "group/project",
			},
			want: "#### project\n\nGit project: [group/project](https://forge.example/group/project)",
		},
		{
			name: "Bitbucket Pipelines",
			env: map[string]string{
				"BITBUCKET_GIT_HTTP_ORIGIN": "https://bitbucket.example/workspace/project.git",
				"BITBUCKET_REPO_FULL_NAME":  "workspace/project",
				"BITBUCKET_REPO_SLUG":       "project",
			},
			want: "#### project\n\nGit project: [workspace/project](https://bitbucket.example/workspace/project)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearProjectEnv(t)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			got, err := BuildHeader(HeaderConfig{Root: t.TempDir()})
			if err != nil {
				t.Fatalf("BuildHeader: %v", err)
			}

			if string(got) != tc.want {
				t.Fatalf("BuildHeader = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildHeaderExplicitValuesOverrideCI(t *testing.T) {
	clearProjectEnv(t)
	t.Setenv("CI_PROJECT_TITLE", "CI project")
	t.Setenv("CI_PROJECT_PATH", "ci/project")
	t.Setenv("CI_PROJECT_URL", "https://ci.example/project")

	got, err := BuildHeader(HeaderConfig{
		Root:       t.TempDir(),
		Title:      "Explicit title",
		SourceName: "explicit/project",
		SourceURL:  "https://example.com/project",
	})
	if err != nil {
		t.Fatalf("BuildHeader: %v", err)
	}

	want := "#### Explicit title\n\nGit project: [explicit/project](https://example.com/project)"
	if string(got) != want {
		t.Fatalf("BuildHeader = %q, want %q", got, want)
	}
}

// clearProjectEnv removes all CI metadata used by project discovery.
func clearProjectEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"CI_PROJECT_PATH",
		"CI_PROJECT_TITLE",
		"CI_PROJECT_URL",
		"GITHUB_REPOSITORY",
		"GITHUB_SERVER_URL",
		"BITBUCKET_GIT_HTTP_ORIGIN",
		"BITBUCKET_REPO_FULL_NAME",
		"BITBUCKET_REPO_SLUG",
	} {
		t.Setenv(key, "")
	}
}
