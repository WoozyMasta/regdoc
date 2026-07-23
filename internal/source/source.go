// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package source discovers project metadata and commit-pinned Markdown link/image base URLs
// from CI environment variables.
// Each supported provider is selected by its own CI sentinel;
// once selected, an incomplete profile never falls through to another provider.
package source

// Source is CI-discovered project metadata and, when a complete profile is available,
// base URLs for rewriting relative Markdown link and image destinations.
type Source struct {
	Name         string // Name is the repository name/path, for header metadata.
	Title        string // Title is a display title, for header metadata.
	ProjectURL   string // ProjectURL is the project web URL, for header metadata.
	LinkBaseURL  string // LinkBaseURL prefixes relative Markdown link destinations. Empty unless the profile is complete.
	ImageBaseURL string // ImageBaseURL prefixes relative Markdown image destinations. Empty unless the profile is complete.
}

// EnvGetter looks up an environment variable, returning "" when unset.
type EnvGetter func(key string) string

// detector inspects env for one CI provider's sentinel.
// ok reports whether the sentinel matched,
// independent of whether the resulting Source fields are fully populated.
type detector func(getenv EnvGetter) (Source, bool)

// providerOrder is the deterministic dispatch order.
// Forgejo and Gitea must be checked before GitHub because both alias GITHUB_* variables
// (and, on Forgejo Runner v7+, GITHUB_ACTIONS=true itself).
var providerOrder = []detector{
	detectGitLab,
	detectBitbucket,
	detectForgejo,
	detectGitea,
	detectGitHub,
	detectWoodpecker,
}

// Resolve dispatches to the first CI provider whose sentinel matches getenv.
// Once a provider is selected, Resolve never falls through to a later provider,
// even when the selected provider's own metadata or commit SHA is incomplete.
// It returns a zero Source when no sentinel matches.
func Resolve(getenv EnvGetter) Source {
	for _, detect := range providerOrder {
		if src, ok := detect(getenv); ok {
			return src
		}
	}

	return Source{}
}
