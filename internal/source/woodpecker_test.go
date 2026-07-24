// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import "testing"

func TestDetectWoodpecker(t *testing.T) {
	cases := []struct {
		forgeType   string
		wantLink    string
		wantImage   string
		wantRelease string
	}{
		{"github", "https://forge.example/group/project/blob/0123456789abcdef/", "https://forge.example/group/project/raw/0123456789abcdef/", "https://forge.example/group/project/releases/tag/"},
		{"gitlab", "https://forge.example/group/project/-/blob/0123456789abcdef/", "https://forge.example/group/project/-/raw/0123456789abcdef/", "https://forge.example/group/project/-/tags/"},
		{"gitea", "https://forge.example/group/project/src/commit/0123456789abcdef/", "https://forge.example/group/project/raw/commit/0123456789abcdef/", "https://forge.example/group/project/src/tag/"},
		{"forgejo", "https://forge.example/group/project/src/commit/0123456789abcdef/", "https://forge.example/group/project/raw/commit/0123456789abcdef/", "https://forge.example/group/project/src/tag/"},
		{"bitbucket", "https://forge.example/group/project/src/0123456789abcdef/", "https://forge.example/group/project/raw/0123456789abcdef/", "https://forge.example/group/project/src/"},
		{"bitbucket_dc", "https://forge.example/group/project/src/0123456789abcdef/", "https://forge.example/group/project/raw/0123456789abcdef/", "https://forge.example/group/project/src/"},
	}

	for _, tc := range cases {
		t.Run(tc.forgeType, func(t *testing.T) {
			got, ok := detectWoodpecker(envMap(map[string]string{
				"CI_FORGE_TYPE": tc.forgeType,
				"CI_REPO_URL":   "https://forge.example/group/project",
				"CI_COMMIT_SHA": "0123456789abcdef",
			}))
			if !ok {
				t.Fatal("expected sentinel match")
			}
			if got.LinkBaseURL != tc.wantLink || got.ImageBaseURL != tc.wantImage {
				t.Fatalf("got link=%q image=%q, want link=%q image=%q",
					got.LinkBaseURL, got.ImageBaseURL, tc.wantLink, tc.wantImage)
			}
			if got.ReleaseBaseURL != tc.wantRelease {
				t.Fatalf("got ReleaseBaseURL = %q, want %q", got.ReleaseBaseURL, tc.wantRelease)
			}
			if got.Name != "project" || got.ProjectURL != "https://forge.example/group/project" {
				t.Fatalf("got %+v, unexpected header metadata", got)
			}
		})
	}
}

func TestDetectWoodpeckerUnrecognizedForgeTypeKeepsHeaderMetadataOnly(t *testing.T) {
	got, ok := detectWoodpecker(envMap(map[string]string{
		"CI_FORGE_TYPE": "some-future-forge",
		"CI_REPO_URL":   "https://forge.example/group/project",
		"CI_COMMIT_SHA": "0123456789abcdef",
	}))
	if !ok {
		t.Fatal("expected sentinel match")
	}
	if got.LinkBaseURL != "" || got.ImageBaseURL != "" || got.ReleaseBaseURL != "" {
		t.Fatalf("got %+v, want empty bases for an unrecognized forge type", got)
	}
	if got.ProjectURL != "https://forge.example/group/project" {
		t.Fatalf("got ProjectURL %q, want header metadata still populated", got.ProjectURL)
	}
}

func TestDetectWoodpeckerSentinelAbsent(t *testing.T) {
	_, ok := detectWoodpecker(envMap(map[string]string{
		"CI_REPO_URL":   "https://forge.example/group/project",
		"CI_COMMIT_SHA": "0123456789abcdef",
	}))
	if ok {
		t.Fatal("expected no match without CI_FORGE_TYPE")
	}
}
