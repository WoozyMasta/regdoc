// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import "testing"

func TestDetectGitLab(t *testing.T) {
	got, ok := detectGitLab(envMap(map[string]string{
		"GITLAB_CI":        "true",
		"CI_PROJECT_PATH":  "group/project",
		"CI_PROJECT_TITLE": "Project",
		"CI_PROJECT_URL":   "https://gitlab.example/group/project",
		"CI_COMMIT_SHA":    "0123456789abcdef",
	}))
	if !ok {
		t.Fatal("expected sentinel match")
	}

	want := Source{
		Name:           "group/project",
		Title:          "Project",
		ProjectURL:     "https://gitlab.example/group/project",
		LinkBaseURL:    "https://gitlab.example/group/project/-/blob/0123456789abcdef/",
		ImageBaseURL:   "https://gitlab.example/group/project/-/raw/0123456789abcdef/",
		ReleaseBaseURL: "https://gitlab.example/group/project/-/tags/",
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDetectGitLabIncompleteProfileLeavesBasesEmpty(t *testing.T) {
	got, ok := detectGitLab(envMap(map[string]string{
		"GITLAB_CI":       "true",
		"CI_PROJECT_PATH": "group/project",
		"CI_PROJECT_URL":  "https://gitlab.example/group/project",
		// CI_COMMIT_SHA intentionally absent.
	}))
	if !ok {
		t.Fatal("expected sentinel match even with incomplete metadata")
	}
	if got.LinkBaseURL != "" || got.ImageBaseURL != "" {
		t.Fatalf("got %+v, want empty bases", got)
	}
	if got.Name != "group/project" {
		t.Fatalf("got Name %q, want header metadata still populated", got.Name)
	}
	if want := "https://gitlab.example/group/project/-/tags/"; got.ReleaseBaseURL != want {
		t.Fatalf("got ReleaseBaseURL %q, want %q (does not require a commit SHA)", got.ReleaseBaseURL, want)
	}
}

func TestDetectGitLabSentinelAbsent(t *testing.T) {
	_, ok := detectGitLab(envMap(map[string]string{
		"CI_PROJECT_URL": "https://gitlab.example/group/project",
		"CI_COMMIT_SHA":  "0123456789abcdef",
	}))
	if ok {
		t.Fatal("expected no match without GITLAB_CI=true")
	}
}
