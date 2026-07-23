// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/google/licensecheck"
)

// licenseCoverageThreshold is the minimum license text match percentage.
const licenseCoverageThreshold = 40.0

// HeaderConfig configures the header prepended to the merged document.
type HeaderConfig struct {
	Root       string // Root is used for automatic metadata discovery.
	Title      string // Title overrides the discovered project title.
	SourceName string // SourceName overrides the discovered project name.
	SourceURL  string // SourceURL overrides the discovered project URL.
	Author     string // Author is included in the generated header.
	Copyright  string // Copyright is included in the generated header.
	License    string // License is an explicit path to a license file. Empty auto-discover a "LICENSE" file.
}

// BuildHeader builds the header content prepended to the merged document.
// It returns an empty slice (not an error) when nothing was configured and nothing could be auto-detected.
func BuildHeader(cfg HeaderConfig) ([]byte, error) {
	project := discoverProject()
	title := firstNonEmpty(cfg.Title, project.Title)
	sourceName := firstNonEmpty(cfg.SourceName, project.Name)
	sourceURL := firstNonEmpty(cfg.SourceURL, project.URL)

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

// project contains CI-discovered metadata used by the generated header.
type project struct {
	Name  string
	Title string
	URL   string
}

// discoverProject returns metadata from the first recognized CI environment.
func discoverProject() project {
	if result := gitLabProject(); result != (project{}) {
		return result
	}

	if result := githubActionsProject(); result != (project{}) {
		return result
	}

	return bitbucketProject()
}

// gitLabProject reads standard GitLab CI project metadata.
func gitLabProject() project {
	return project{
		Name:  os.Getenv("CI_PROJECT_PATH"),
		Title: os.Getenv("CI_PROJECT_TITLE"),
		URL:   os.Getenv("CI_PROJECT_URL"),
	}
}

// githubActionsProject reads GitHub Actions metadata
// also provided by Gitea and Forgejo Actions.
func githubActionsProject() project {
	serverURL := strings.TrimSuffix(os.Getenv("GITHUB_SERVER_URL"), "/")
	repository := strings.Trim(os.Getenv("GITHUB_REPOSITORY"), "/")
	if serverURL == "" || repository == "" {
		return project{}
	}

	return project{
		Name:  repository,
		Title: path.Base(repository),
		URL:   serverURL + "/" + repository,
	}
}

// bitbucketProject reads standard Bitbucket Pipelines repository metadata.
func bitbucketProject() project {
	origin := strings.TrimSuffix(strings.TrimSuffix(os.Getenv("BITBUCKET_GIT_HTTP_ORIGIN"), "/"), ".git")
	name := os.Getenv("BITBUCKET_REPO_FULL_NAME")
	title := os.Getenv("BITBUCKET_REPO_SLUG")
	if name == "" {
		name = title
	}
	if title == "" && origin != "" {
		title = path.Base(origin)
	}

	return project{Name: name, Title: title, URL: origin}
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
