// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

// detectGitLab reads standard GitLab CI project metadata.
func detectGitLab(getenv EnvGetter) (Source, bool) {
	if getenv("GITLAB_CI") != "true" {
		return Source{}, false
	}

	projectURL := getenv("CI_PROJECT_URL")
	link, image := buildRoutes(forgeGitLab, projectURL, getenv("CI_COMMIT_SHA"))

	return Source{
		Name:         getenv("CI_PROJECT_PATH"),
		Title:        getenv("CI_PROJECT_TITLE"),
		ProjectURL:   projectURL,
		LinkBaseURL:  link,
		ImageBaseURL: image,
	}, true
}
