// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package quay publishes a repository description to the Quay API via an OAuth bearer token.
package quay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/woozymasta/regdoc/internal/httpx"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

// tagListPageSize is the number of tags requested per Quay API page.
const tagListPageSize = 100

// Client publishes repository descriptions to the Quay API.
type Client struct {
	HTTP   *http.Client // HTTP executes Quay API requests.
	Scheme string       // Scheme is "https" or "http" (only for an explicitly allowed plain-http registry).
	Token  string       // Token is sent as OAuth bearer authentication.
}

// New builds a Quay Client.
func New(httpClient *http.Client, scheme, token string) *Client {
	return &Client{HTTP: httpClient, Scheme: scheme, Token: token}
}

// Publish updates tgt's description on Quay.
// Quay has a single description field;
// ShortDescription is not supported by the API and is ignored.
func (c *Client) Publish(ctx context.Context, tgt target.Target, doc provider.Document) error {
	payload, err := json.Marshal(map[string]string{"description": string(doc.Content)})
	if err != nil {
		return fmt.Errorf("encode quay request body: %w", err)
	}

	url := c.Scheme + "://" + tgt.Registry + "/api/v1/repository/" + tgt.Repository

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build quay request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("quay: update description: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !httpx.IsSuccess(resp.StatusCode) {
		return httpx.NewHTTPError(provider.Quay, resp)
	}

	return nil
}

// ListTags returns every active tag currently published for tgt's repository,
// using the same OAuth bearer token Publish already uses.
func (c *Client) ListTags(ctx context.Context, tgt target.Target) ([]string, error) {
	var tags []string

	for page := 1; ; page++ {
		query := url.Values{}
		query.Set("onlyActiveTags", "true")
		query.Set("page", strconv.Itoa(page))
		query.Set("limit", strconv.Itoa(tagListPageSize))

		reqURL := c.Scheme + "://" + tgt.Registry + "/api/v1/repository/" + tgt.Repository + "/tag/?" + query.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build quay tags request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.Token)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("quay: list tags: %w", err)
		}

		if !httpx.IsSuccess(resp.StatusCode) {
			httpErr := httpx.NewHTTPError(provider.Quay, resp)
			_ = resp.Body.Close()

			return nil, httpErr
		}

		var result struct {
			Tags []struct {
				Name string `json:"name"`
			} `json:"tags"`
			HasAdditional bool `json:"has_additional"`
		}

		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, httpx.ErrorBodyLimit)).Decode(&result)
		_ = resp.Body.Close()

		if decodeErr != nil {
			return nil, fmt.Errorf("quay: decode tags response: %w", decodeErr)
		}

		for _, t := range result.Tags {
			tags = append(tags, t.Name)
		}

		if !result.HasAdditional {
			return tags, nil
		}
	}
}
