// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import (
	"path"
	"strings"
)

// detectGitHub reads GitHub Actions metadata.
func detectGitHub(getenv EnvGetter) (Source, bool) {
	if getenv("GITHUB_ACTIONS") != "true" {
		return Source{}, false
	}

	return githubCompatibleSource(getenv, forgeGitHub), true
}

// githubCompatibleSource reads the GITHUB_SERVER_URL/GITHUB_REPOSITORY/GITHUB_SHA
// triple shared by GitHub Actions itself,
// by Gitea Actions (which exposes no native variable names for these values),
// and by Forgejo Actions as a per-field fallback when native FORGEJO_* variables are unset.
func githubCompatibleSource(getenv EnvGetter, kind forgeKind) Source {
	serverURL := strings.TrimSuffix(getenv("GITHUB_SERVER_URL"), "/")
	repository := strings.Trim(getenv("GITHUB_REPOSITORY"), "/")

	var projectURL, title string
	if serverURL != "" && repository != "" {
		projectURL = serverURL + "/" + repository
		title = path.Base(repository)
	}

	link, image := buildRoutes(kind, projectURL, getenv("GITHUB_SHA"))

	return Source{
		Name:           repository,
		Title:          title,
		ProjectURL:     projectURL,
		LinkBaseURL:    link,
		ImageBaseURL:   image,
		ReleaseBaseURL: releaseBaseURL(kind, projectURL),
	}
}
