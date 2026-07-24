// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import (
	"path"
	"strings"
)

// detectForgejo reads Forgejo Actions metadata. FORGEJO_ACTIONS is only set by Forgejo Runner v7.0.0+;
// older runners never set it, so their jobs are indistinguishable from plain GitHub-compatible variables
// and are matched by detectGitHub instead.
// This is a documented limitation with no code workaround:
// no distinguishing signal exists on older runners.
func detectForgejo(getenv EnvGetter) (Source, bool) {
	if getenv("FORGEJO_ACTIONS") != "true" {
		return Source{}, false
	}

	serverURL := strings.TrimSuffix(firstNonEmptyEnv(getenv, "FORGEJO_SERVER_URL", "GITHUB_SERVER_URL"), "/")
	repository := strings.Trim(firstNonEmptyEnv(getenv, "FORGEJO_REPOSITORY", "GITHUB_REPOSITORY"), "/")
	sha := firstNonEmptyEnv(getenv, "FORGEJO_SHA", "GITHUB_SHA")

	var projectURL, title string
	if serverURL != "" && repository != "" {
		projectURL = serverURL + "/" + repository
		title = path.Base(repository)
	}

	link, image := buildRoutes(forgeGiteaLike, projectURL, sha)

	return Source{
		Name:           repository,
		Title:          title,
		ProjectURL:     projectURL,
		LinkBaseURL:    link,
		ImageBaseURL:   image,
		ReleaseBaseURL: releaseBaseURL(forgeGiteaLike, projectURL),
	}, true
}

// firstNonEmptyEnv returns the value of the first key that is non-empty.
func firstNonEmptyEnv(getenv EnvGetter, keys ...string) string {
	for _, key := range keys {
		if v := getenv(key); v != "" {
			return v
		}
	}

	return ""
}
