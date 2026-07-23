// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package version holds build metadata for the regdoc CLI.
package version

// name is the product token in the HTTP User-Agent.
const name = "regdoc"

// Build metadata injected with -ldflags.
var (
	Version   string
	Commit    string
	BuildTime string
	URL       string
)

// UserAgent returns the HTTP User-Agent for this build.
func UserAgent() string {
	version := Version
	if version == "" {
		version = "dev"
	}

	return name + "/" + version
}
