// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import "testing"

func TestDetectForgejoNativeVariablesWinOverGitHubAlias(t *testing.T) {
	got, ok := detectForgejo(envMap(map[string]string{
		"FORGEJO_ACTIONS":    "true",
		"FORGEJO_SERVER_URL": "https://forgejo.example",
		"FORGEJO_REPOSITORY": "group/project",
		"FORGEJO_SHA":        "0123456789abcdef",
		"GITHUB_SERVER_URL":  "https://github.com",
		"GITHUB_REPOSITORY":  "other/project",
		"GITHUB_SHA":         "fedcba9876543210",
	}))
	if !ok {
		t.Fatal("expected sentinel match")
	}

	want := Source{
		Name:         "group/project",
		Title:        "project",
		ProjectURL:   "https://forgejo.example/group/project",
		LinkBaseURL:  "https://forgejo.example/group/project/src/commit/0123456789abcdef/",
		ImageBaseURL: "https://forgejo.example/group/project/raw/commit/0123456789abcdef/",
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDetectForgejoFallsBackToGitHubAliasesPerField(t *testing.T) {
	// FORGEJO_ACTIONS=true but no native FORGEJO_* variables:
	// falls back to the GITHUB_*-compatible aliases Forgejo also mirrors.
	got, ok := detectForgejo(envMap(map[string]string{
		"FORGEJO_ACTIONS":   "true",
		"GITHUB_SERVER_URL": "https://forgejo.example",
		"GITHUB_REPOSITORY": "group/project",
		"GITHUB_SHA":        "0123456789abcdef",
	}))
	if !ok {
		t.Fatal("expected sentinel match")
	}

	want := Source{
		Name:         "group/project",
		Title:        "project",
		ProjectURL:   "https://forgejo.example/group/project",
		LinkBaseURL:  "https://forgejo.example/group/project/src/commit/0123456789abcdef/",
		ImageBaseURL: "https://forgejo.example/group/project/raw/commit/0123456789abcdef/",
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDetectForgejoSentinelAbsentOnOlderRunners(t *testing.T) {
	// Forgejo Runner < v7.0.0 never sets FORGEJO_ACTIONS:
	// this job is indistinguishable from GitHub-compatible variables at this detector's level.
	// Documented limitation: package-level Resolve picks it up via detectGitHub instead
	// (see TestResolveGiteaAndForgejoWinOverGitHubAlias for the case where the sentinel IS present).
	_, ok := detectForgejo(envMap(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_SERVER_URL": "https://forgejo.example",
		"GITHUB_REPOSITORY": "group/project",
		"GITHUB_SHA":        "0123456789abcdef",
	}))
	if ok {
		t.Fatal("expected no match without FORGEJO_ACTIONS=true")
	}
}
