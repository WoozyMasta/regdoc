// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import "testing"

func TestDetectBitbucket(t *testing.T) {
	got, ok := detectBitbucket(envMap(map[string]string{
		"BITBUCKET_BUILD_NUMBER":    "42",
		"BITBUCKET_GIT_HTTP_ORIGIN": "https://bitbucket.example/workspace/project.git",
		"BITBUCKET_REPO_FULL_NAME":  "workspace/project",
		"BITBUCKET_REPO_SLUG":       "project",
		"BITBUCKET_COMMIT":          "0123456789abcdef",
	}))
	if !ok {
		t.Fatal("expected sentinel match")
	}

	want := Source{
		Name:           "workspace/project",
		Title:          "project",
		ProjectURL:     "https://bitbucket.example/workspace/project",
		LinkBaseURL:    "https://bitbucket.example/workspace/project/src/0123456789abcdef/",
		ImageBaseURL:   "https://bitbucket.example/workspace/project/raw/0123456789abcdef/",
		ReleaseBaseURL: "https://bitbucket.example/workspace/project/src/",
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDetectBitbucketSentinelAbsent(t *testing.T) {
	_, ok := detectBitbucket(envMap(map[string]string{
		"BITBUCKET_GIT_HTTP_ORIGIN": "https://bitbucket.example/workspace/project.git",
		"BITBUCKET_COMMIT":          "0123456789abcdef",
	}))
	if ok {
		t.Fatal("expected no match without BITBUCKET_BUILD_NUMBER")
	}
}

func TestDetectBitbucketIncompleteProfileLeavesBasesEmpty(t *testing.T) {
	got, ok := detectBitbucket(envMap(map[string]string{
		"BITBUCKET_BUILD_NUMBER": "42",
		// BITBUCKET_GIT_HTTP_ORIGIN and BITBUCKET_COMMIT intentionally absent.
	}))
	if !ok {
		t.Fatal("expected sentinel match even with incomplete metadata")
	}
	if got.LinkBaseURL != "" || got.ImageBaseURL != "" {
		t.Fatalf("got %+v, want empty bases", got)
	}
	if got.ReleaseBaseURL != "" {
		t.Fatalf("got ReleaseBaseURL %q, want empty (no origin known)", got.ReleaseBaseURL)
	}
}
