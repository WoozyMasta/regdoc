// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import (
	"path"
	"strings"
)

// detectWoodpecker reads Woodpecker CI metadata.
// Woodpecker self-reports which forge it is connected to via CI_FORGE_TYPE
// and exposes a uniform repository URL and commit SHA regardless of forge,
// so the same route builder used by the native per-forge detectors applies here too.
func detectWoodpecker(getenv EnvGetter) (Source, bool) {
	forgeType := getenv("CI_FORGE_TYPE")
	if forgeType == "" {
		return Source{}, false
	}

	projectURL := strings.TrimSuffix(getenv("CI_REPO_URL"), "/")

	var name string
	if projectURL != "" {
		name = path.Base(projectURL)
	}

	var link, image string
	if kind, known := woodpeckerForgeKind(forgeType); known {
		link, image = buildRoutes(kind, projectURL, getenv("CI_COMMIT_SHA"))
	}

	return Source{
		Name:         name,
		Title:        name,
		ProjectURL:   projectURL,
		LinkBaseURL:  link,
		ImageBaseURL: image,
	}, true
}

// woodpeckerForgeKind maps a documented CI_FORGE_TYPE value to the matching route shape.
// An unrecognized value still yields header metadata (see detectWoodpecker) but no link/image bases.
func woodpeckerForgeKind(forgeType string) (forgeKind, bool) {
	switch forgeType {
	case "github":
		return forgeGitHub, true

	case "gitlab":
		return forgeGitLab, true

	case "gitea", "forgejo":
		return forgeGiteaLike, true

	case "bitbucket", "bitbucket_dc":
		return forgeBitbucket, true

	default:
		return 0, false
	}
}
