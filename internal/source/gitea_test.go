// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import "testing"

func TestDetectGitea(t *testing.T) {
	got, ok := detectGitea(envMap(map[string]string{
		"GITEA_ACTIONS":     "true",
		"GITHUB_SERVER_URL": "https://gitea.example",
		"GITHUB_REPOSITORY": "group/project",
		"GITHUB_SHA":        "0123456789abcdef",
	}))
	if !ok {
		t.Fatal("expected sentinel match")
	}

	want := Source{
		Name:         "group/project",
		Title:        "project",
		ProjectURL:   "https://gitea.example/group/project",
		LinkBaseURL:  "https://gitea.example/group/project/src/commit/0123456789abcdef/",
		ImageBaseURL: "https://gitea.example/group/project/raw/commit/0123456789abcdef/",
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDetectGiteaSentinelAbsent(t *testing.T) {
	// Only GITHUB_*-compatible variables set: without GITEA_ACTIONS=true this must not match,
	// since Gitea has no native variables of its own to distinguish it from a real GitHub Actions job.
	_, ok := detectGitea(envMap(map[string]string{
		"GITHUB_SERVER_URL": "https://gitea.example",
		"GITHUB_REPOSITORY": "group/project",
		"GITHUB_SHA":        "0123456789abcdef",
	}))
	if ok {
		t.Fatal("expected no match without GITEA_ACTIONS=true")
	}
}

func TestDetectGiteaIncompleteProfileLeavesBasesEmpty(t *testing.T) {
	got, ok := detectGitea(envMap(map[string]string{
		"GITEA_ACTIONS":     "true",
		"GITHUB_SERVER_URL": "https://gitea.example",
		// GITHUB_REPOSITORY and GITHUB_SHA intentionally absent.
	}))
	if !ok {
		t.Fatal("expected sentinel match even with incomplete metadata")
	}
	if got.LinkBaseURL != "" || got.ImageBaseURL != "" {
		t.Fatalf("got %+v, want empty bases", got)
	}
}
