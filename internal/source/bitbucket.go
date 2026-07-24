// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package source

import (
	"path"
	"strings"
)

// detectBitbucket reads Bitbucket Pipelines metadata.
// Bitbucket documents no single boolean sentinel;
// BITBUCKET_BUILD_NUMBER is always set on every Pipelines run and serves that role here.
func detectBitbucket(getenv EnvGetter) (Source, bool) {
	if getenv("BITBUCKET_BUILD_NUMBER") == "" {
		return Source{}, false
	}

	origin := strings.TrimSuffix(strings.TrimSuffix(getenv("BITBUCKET_GIT_HTTP_ORIGIN"), "/"), ".git")
	name := getenv("BITBUCKET_REPO_FULL_NAME")
	title := getenv("BITBUCKET_REPO_SLUG")

	if name == "" {
		name = title
	}
	if title == "" && origin != "" {
		title = path.Base(origin)
	}

	link, image := buildRoutes(forgeBitbucket, origin, getenv("BITBUCKET_COMMIT"))

	return Source{
		Name:           name,
		Title:          title,
		ProjectURL:     origin,
		LinkBaseURL:    link,
		ImageBaseURL:   image,
		ReleaseBaseURL: releaseBaseURL(forgeBitbucket, origin),
	}, true
}
