// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package harbor publishes a repository description to the Harbor REST API (v2.0) via HTTP Basic authentication.
package harbor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/woozymasta/regdoc/internal/httpx"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

// tagListPageSize is the number of artifacts requested per Harbor API page.
const tagListPageSize = 100

// Client publishes repository descriptions to the Harbor API.
type Client struct {
	HTTP     *http.Client // HTTP executes Harbor API requests.
	Scheme   string       // Scheme is "https" or "http" (only for an explicitly allowed plain-http registry).
	Username string       // Username is sent as Basic authentication.
	Password string       // Password is sent as Basic authentication.
}

// New builds a Harbor Client.
func New(httpClient *http.Client, scheme, username, password string) *Client {
	return &Client{HTTP: httpClient, Scheme: scheme, Username: username, Password: password}
}

// Publish updates tgt's description on Harbor.
func (c *Client) Publish(ctx context.Context, tgt target.Target, doc provider.Document) error {
	project, repository, err := splitRepository(tgt.Repository)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]string{"description": string(doc.Content)})
	if err != nil {
		return fmt.Errorf("encode harbor request body: %w", err)
	}

	reqURL := c.Scheme + "://" + tgt.Registry + "/api/v2.0/projects/" +
		url.PathEscape(project) + "/repositories/" + url.PathEscape(repository)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build harbor request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.Username, c.Password)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("harbor: update description: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !httpx.IsSuccess(resp.StatusCode) {
		return httpx.NewHTTPError(provider.Harbor, resp)
	}

	return nil
}

// ListTags returns every tag currently published for tgt's repository,
// flattened across all artifacts (an artifact/digest can carry multiple tags),
// using the same Basic authentication Publish already uses.
func (c *Client) ListTags(ctx context.Context, tgt target.Target) ([]string, error) {
	project, repository, err := splitRepository(tgt.Repository)
	if err != nil {
		return nil, err
	}

	var tags []string

	for page := 1; ; page++ {
		query := url.Values{}
		query.Set("with_tag", "true")
		query.Set("page", strconv.Itoa(page))
		query.Set("page_size", strconv.Itoa(tagListPageSize))

		reqURL := c.Scheme + "://" + tgt.Registry + "/api/v2.0/projects/" +
			url.PathEscape(project) + "/repositories/" + url.PathEscape(repository) +
			"/artifacts?" + query.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build harbor tags request: %w", err)
		}

		req.SetBasicAuth(c.Username, c.Password)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("harbor: list tags: %w", err)
		}

		if !httpx.IsSuccess(resp.StatusCode) {
			httpErr := httpx.NewHTTPError(provider.Harbor, resp)
			_ = resp.Body.Close()

			return nil, httpErr
		}

		var artifacts []struct {
			Tags []struct {
				Name string `json:"name"`
			} `json:"tags"`
		}

		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, httpx.APIResponseBodyLimit)).Decode(&artifacts)
		_ = resp.Body.Close()

		if decodeErr != nil {
			return nil, fmt.Errorf("harbor: decode artifacts response: %w", decodeErr)
		}

		for _, artifact := range artifacts {
			for _, t := range artifact.Tags {
				tags = append(tags, t.Name)
			}
		}

		if len(artifacts) < tagListPageSize {
			return tags, nil
		}
	}
}

// splitRepository separates the Harbor project from its repository path.
func splitRepository(repository string) (project, repo string, err error) {
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("harbor repository %q must include a project: project/repository", repository)
	}

	return parts[0], parts[1], nil
}
