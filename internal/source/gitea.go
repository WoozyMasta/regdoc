// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

// detectGitea reads Gitea Actions metadata.
// Gitea exposes no native GITEA_SERVER_URL/GITEA_REPOSITORY/GITEA_SHA variables:
// only the GITHUB_*-compatible aliases carry server, repository and commit data,
// even though GITEA_ACTIONS itself is a Gitea-native sentinel.
func detectGitea(getenv EnvGetter) (Source, bool) {
	if getenv("GITEA_ACTIONS") != "true" {
		return Source{}, false
	}

	return githubCompatibleSource(getenv, forgeGiteaLike), true
}
