// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import "strings"

// forgeKind selects the web route shape used to build commit-pinned URLs.
type forgeKind int

// Supported route shapes.
const (
	forgeGitHub forgeKind = iota
	forgeGitLab
	forgeGiteaLike // Gitea and Forgejo share identical src/commit and raw/commit routes.
	forgeBitbucket
)

// buildRoutes returns commit-pinned link and image base URLs for kind.
// It returns ("", "") when projectURL or sha is empty:
// callers must never substitute a branch name or otherwise guess at a route.
func buildRoutes(kind forgeKind, projectURL, sha string) (link, image string) {
	projectURL = strings.TrimSuffix(projectURL, "/")
	if projectURL == "" || sha == "" {
		return "", ""
	}

	switch kind {
	case forgeGitLab:
		return projectURL + "/-/blob/" + sha + "/", projectURL + "/-/raw/" + sha + "/"

	case forgeGitHub:
		return projectURL + "/blob/" + sha + "/", projectURL + "/raw/" + sha + "/"

	case forgeGiteaLike:
		return projectURL + "/src/commit/" + sha + "/", projectURL + "/raw/commit/" + sha + "/"

	case forgeBitbucket:
		return projectURL + "/src/" + sha + "/", projectURL + "/raw/" + sha + "/"

	default:
		return "", ""
	}
}
