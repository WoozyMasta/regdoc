// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import "testing"

func TestBuildRoutes(t *testing.T) {
	const sha = "0123456789abcdef"

	cases := []struct {
		name       string
		kind       forgeKind
		projectURL string
		wantLink   string
		wantImage  string
	}{
		{
			name:       "GitLab",
			kind:       forgeGitLab,
			projectURL: "https://gitlab.example/group/project",
			wantLink:   "https://gitlab.example/group/project/-/blob/" + sha + "/",
			wantImage:  "https://gitlab.example/group/project/-/raw/" + sha + "/",
		},
		{
			name:       "GitHub",
			kind:       forgeGitHub,
			projectURL: "https://github.com/group/project",
			wantLink:   "https://github.com/group/project/blob/" + sha + "/",
			wantImage:  "https://github.com/group/project/raw/" + sha + "/",
		},
		{
			name:       "Gitea/Forgejo",
			kind:       forgeGiteaLike,
			projectURL: "https://gitea.example/group/project",
			wantLink:   "https://gitea.example/group/project/src/commit/" + sha + "/",
			wantImage:  "https://gitea.example/group/project/raw/commit/" + sha + "/",
		},
		{
			name:       "Bitbucket",
			kind:       forgeBitbucket,
			projectURL: "https://bitbucket.example/workspace/project",
			wantLink:   "https://bitbucket.example/workspace/project/src/" + sha + "/",
			wantImage:  "https://bitbucket.example/workspace/project/raw/" + sha + "/",
		},
		{
			name:       "trailing slash trimmed",
			kind:       forgeGitLab,
			projectURL: "https://gitlab.example/group/project/",
			wantLink:   "https://gitlab.example/group/project/-/blob/" + sha + "/",
			wantImage:  "https://gitlab.example/group/project/-/raw/" + sha + "/",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			link, image := buildRoutes(tc.kind, tc.projectURL, sha)
			if link != tc.wantLink || image != tc.wantImage {
				t.Fatalf("buildRoutes = (%q, %q), want (%q, %q)", link, image, tc.wantLink, tc.wantImage)
			}
		})
	}
}

func TestBuildRoutesEmptyInputsYieldEmptyRoutes(t *testing.T) {
	cases := []struct {
		name       string
		projectURL string
		sha        string
	}{
		{"empty project URL", "", "0123456789abcdef"},
		{"empty sha", "https://gitlab.example/group/project", ""},
		{"both empty", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			link, image := buildRoutes(forgeGitLab, tc.projectURL, tc.sha)
			if link != "" || image != "" {
				t.Fatalf("buildRoutes = (%q, %q), want empty", link, image)
			}
		})
	}
}
