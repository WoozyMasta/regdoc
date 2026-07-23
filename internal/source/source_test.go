// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import "testing"

// envMap builds an EnvGetter backed by a fixed map, returning "" for anything not listed.
func envMap(vars map[string]string) EnvGetter {
	return func(key string) string { return vars[key] }
}

func TestResolveNoSentinelsReturnsZeroSource(t *testing.T) {
	got := Resolve(envMap(nil))
	if got != (Source{}) {
		t.Fatalf("Resolve = %+v, want zero Source", got)
	}
}

func TestResolveIncompleteProfileDoesNotFallThrough(t *testing.T) {
	got := Resolve(envMap(map[string]string{
		"GITLAB_CI": "true",
		// CI_PROJECT_URL and CI_COMMIT_SHA intentionally absent:
		// GitLab is still selected and must not fall through to Bitbucket below.
		"BITBUCKET_BUILD_NUMBER":    "42",
		"BITBUCKET_GIT_HTTP_ORIGIN": "https://bitbucket.example/workspace/project.git",
		"BITBUCKET_COMMIT":          "0123456789abcdef",
	}))

	if got.LinkBaseURL != "" || got.ImageBaseURL != "" {
		t.Fatalf("Resolve = %+v, want empty bases (no fallthrough to Bitbucket)", got)
	}
}

func TestResolveGiteaAndForgejoWinOverGitHubAlias(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		wantLink string
	}{
		{
			name: "Gitea",
			env: map[string]string{
				"GITEA_ACTIONS":     "true",
				"GITHUB_ACTIONS":    "true",
				"GITHUB_SERVER_URL": "https://gitea.example",
				"GITHUB_REPOSITORY": "group/project",
				"GITHUB_SHA":        "0123456789abcdef",
			},
			wantLink: "https://gitea.example/group/project/src/commit/0123456789abcdef/",
		},
		{
			name: "Forgejo",
			env: map[string]string{
				"FORGEJO_ACTIONS":    "true",
				"GITHUB_ACTIONS":     "true",
				"FORGEJO_SERVER_URL": "https://forgejo.example",
				"FORGEJO_REPOSITORY": "group/project",
				"FORGEJO_SHA":        "0123456789abcdef",
				"GITHUB_SERVER_URL":  "https://github.com",
				"GITHUB_REPOSITORY":  "group/project",
				"GITHUB_SHA":         "fedcba9876543210",
			},
			wantLink: "https://forgejo.example/group/project/src/commit/0123456789abcdef/",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Resolve(envMap(tc.env))
			if got.LinkBaseURL != tc.wantLink {
				t.Fatalf("LinkBaseURL = %q, want %q", got.LinkBaseURL, tc.wantLink)
			}
		})
	}
}

func TestResolveGitLabSentinelGatesOnBooleanNotVariablePresence(t *testing.T) {
	// GITLAB_CI is not "true", so detectGitLab must not match even though a CI_PROJECT_URL happens to be set.
	// Resolve should fall through all the way to Woodpecker,
	// which reports the forge via CI_FORGE_TYPE and reads its own CI_REPO_URL/CI_COMMIT_SHA - distinct variables from GitLab's.
	got := Resolve(envMap(map[string]string{
		"CI_PROJECT_URL": "https://real-gitlab.example/group/project",
		"CI_FORGE_TYPE":  "gitlab",
		"CI_REPO_URL":    "https://woodpecker.example/group/project",
		"CI_COMMIT_SHA":  "0123456789abcdef",
	}))

	want := "https://woodpecker.example/group/project/-/blob/0123456789abcdef/"
	if got.LinkBaseURL != want {
		t.Fatalf("LinkBaseURL = %q, want %q (should resolve via Woodpecker, not GitLab)", got.LinkBaseURL, want)
	}
}
