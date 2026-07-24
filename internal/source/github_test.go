// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import "testing"

func TestDetectGitHub(t *testing.T) {
	got, ok := detectGitHub(envMap(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_SERVER_URL": "https://github.com",
		"GITHUB_REPOSITORY": "group/project",
		"GITHUB_SHA":        "0123456789abcdef",
	}))
	if !ok {
		t.Fatal("expected sentinel match")
	}

	want := Source{
		Name:           "group/project",
		Title:          "project",
		ProjectURL:     "https://github.com/group/project",
		LinkBaseURL:    "https://github.com/group/project/blob/0123456789abcdef/",
		ImageBaseURL:   "https://github.com/group/project/raw/0123456789abcdef/",
		ReleaseBaseURL: "https://github.com/group/project/releases/tag/",
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDetectGitHubIncompleteProfileLeavesBasesEmpty(t *testing.T) {
	got, ok := detectGitHub(envMap(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_SERVER_URL": "https://github.com",
		"GITHUB_REPOSITORY": "group/project",
		// GITHUB_SHA intentionally absent.
	}))
	if !ok {
		t.Fatal("expected sentinel match even with incomplete metadata")
	}
	if got.LinkBaseURL != "" || got.ImageBaseURL != "" {
		t.Fatalf("got %+v, want empty bases", got)
	}
	if want := "https://github.com/group/project/releases/tag/"; got.ReleaseBaseURL != want {
		t.Fatalf("got ReleaseBaseURL %q, want %q (does not require a commit SHA)", got.ReleaseBaseURL, want)
	}
}

func TestDetectGitHubSentinelAbsent(t *testing.T) {
	_, ok := detectGitHub(envMap(map[string]string{
		"GITHUB_SERVER_URL": "https://github.com",
		"GITHUB_REPOSITORY": "group/project",
		"GITHUB_SHA":        "0123456789abcdef",
	}))
	if ok {
		t.Fatal("expected no match without GITHUB_ACTIONS=true")
	}
}
