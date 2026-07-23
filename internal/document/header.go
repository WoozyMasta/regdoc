// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/licensecheck"
)

// licenseCoverageThreshold is the minimum license text match percentage.
const licenseCoverageThreshold = 40.0

// HeaderConfig configures the header prepended to the merged document.
type HeaderConfig struct {
	Root            string // Root is used for automatic metadata discovery.
	Title           string // Title overrides DiscoveredTitle.
	SourceName      string // SourceName overrides DiscoveredName.
	SourceURL       string // SourceURL overrides DiscoveredURL.
	Author          string // Author is included in the generated header.
	Copyright       string // Copyright is included in the generated header.
	License         string // License is an explicit path to a license file. Empty auto-discover a "LICENSE" file.
	DiscoveredName  string // DiscoveredName is CI-resolved project metadata; SourceName wins when set.
	DiscoveredTitle string // DiscoveredTitle is CI-resolved project metadata; Title wins when set.
	DiscoveredURL   string // DiscoveredURL is CI-resolved project metadata; SourceURL wins when set.
}

// BuildHeader builds the header content prepended to the merged document.
// It returns an empty slice (not an error) when nothing was configured and nothing could be auto-detected.
func BuildHeader(cfg HeaderConfig) ([]byte, error) {
	title := firstNonEmpty(cfg.Title, cfg.DiscoveredTitle)
	sourceName := firstNonEmpty(cfg.SourceName, cfg.DiscoveredName)
	sourceURL := firstNonEmpty(cfg.SourceURL, cfg.DiscoveredURL)

	license, err := detectLicense(cfg.Root, cfg.License)
	if err != nil {
		return nil, err
	}

	var paragraphs []string

	if title != "" {
		paragraphs = append(paragraphs, "#### "+title)
	}
	if line := projectLine(sourceName, sourceURL); line != "" {
		paragraphs = append(paragraphs, line)
	}
	if cfg.Author != "" {
		paragraphs = append(paragraphs, "Author: "+cfg.Author)
	}
	if cfg.Copyright != "" {
		paragraphs = append(paragraphs, "Copyright: "+cfg.Copyright)
	}
	if license != "" {
		paragraphs = append(paragraphs, "License: "+license)
	}

	if len(paragraphs) == 0 {
		return nil, nil
	}

	return []byte(strings.Join(paragraphs, "\n\n")), nil
}

// projectLine renders project metadata from the available name and URL.
func projectLine(name, url string) string {
	switch {
	case name != "" && url != "":
		return fmt.Sprintf("Git project: [%s](%s)", name, url)

	case name != "":
		return "Git project: " + name

	case url != "":
		return "Git project: " + url

	default:
		return ""
	}
}

// firstNonEmpty returns the first non-empty value.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// detectLicense treats an absent or unrecognized auto-discovered file as no license.
// An explicitly requested file remains a configuration requirement.
func detectLicense(root, explicitPath string) (string, error) {
	path := explicitPath

	if path == "" {
		found, ok, err := FindOptionalFile(root, "LICENSE")
		if err != nil {
			return "", err
		}

		if !ok {
			return "", nil
		}

		path = found
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is user-supplied CLI input or discovered under --root.
	if err != nil {
		return "", fmt.Errorf("read license file %q: %w", path, err)
	}

	cov := licensecheck.Scan(data)
	if cov.Percent < licenseCoverageThreshold || len(cov.Match) == 0 {
		return "", nil
	}

	return cov.Match[0].ID, nil
}
