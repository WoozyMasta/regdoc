// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package dockerhub publishes a repository description to the Docker Hub API (hub.docker.com/v2),
// which is separate from the Distribution Registry API and requires its own JWT login exchange.
package dockerhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/woozymasta/regdoc/internal/httpx"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

// defaultBaseURL is Docker Hub metadata API endpoint.
const defaultBaseURL = "https://hub.docker.com"

// Client publishes repository descriptions to the Docker Hub Hub API.
type Client struct {
	HTTP     *http.Client // HTTP executes Docker Hub API requests.
	BaseURL  string       // BaseURL is the Docker Hub API endpoint.
	Username string       // Username is used for the Hub login exchange.
	Token    string       // Token is the account password or a PAT/organization access token.
}

// New builds a Docker Hub Client.
func New(httpClient *http.Client, username, token string) *Client {
	return &Client{HTTP: httpClient, BaseURL: defaultBaseURL, Username: username, Token: token}
}

// Publish updates tgt's full and short description on Docker Hub.
func (c *Client) Publish(ctx context.Context, tgt target.Target, doc provider.Document) error {
	jwt, err := c.login(ctx)
	if err != nil {
		return err
	}

	body := map[string]string{"full_description": string(doc.Content)}
	if doc.ShortDescription != "" {
		body["description"] = doc.ShortDescription
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode dockerhub request body: %w", err)
	}

	url := c.BaseURL + "/v2/repositories/" + tgt.Repository + "/"

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build dockerhub request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "JWT "+jwt)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("dockerhub: update description: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !httpx.IsSuccess(resp.StatusCode) {
		return httpx.NewHTTPError(provider.DockerHub, resp)
	}

	return nil
}

// ListTags returns every tag currently published for tgt's repository,
// using the same JWT login exchange Publish already performs.
func (c *Client) ListTags(ctx context.Context, tgt target.Target) ([]string, error) {
	jwt, err := c.login(ctx)
	if err != nil {
		return nil, err
	}

	var tags []string

	url := c.BaseURL + "/v2/repositories/" + tgt.Repository + "/tags/?page_size=100"

	for url != "" {
		var page struct {
			Next    string `json:"next"`
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build dockerhub tags request: %w", err)
		}

		req.Header.Set("Authorization", "JWT "+jwt)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("dockerhub: list tags: %w", err)
		}

		if !httpx.IsSuccess(resp.StatusCode) {
			httpErr := httpx.NewHTTPError(provider.DockerHub, resp)
			_ = resp.Body.Close()

			return nil, httpErr
		}

		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, httpx.ErrorBodyLimit)).Decode(&page)
		_ = resp.Body.Close()

		if decodeErr != nil {
			return nil, fmt.Errorf("dockerhub: decode tags response: %w", decodeErr)
		}

		for _, result := range page.Results {
			tags = append(tags, result.Name)
		}

		url = page.Next
	}

	return tags, nil
}

// login uses Docker Hub's login endpoint.
// A Distribution Registry token cannot authorize the Hub API update endpoint.
func (c *Client) login(ctx context.Context) (string, error) {
	payload, err := json.Marshal(map[string]string{"username": c.Username, "password": c.Token})
	if err != nil {
		return "", fmt.Errorf("encode dockerhub login request: %w", err)
	}

	url := c.BaseURL + "/v2/users/login/"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build dockerhub login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("dockerhub: login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !httpx.IsSuccess(resp.StatusCode) {
		return "", httpx.NewHTTPError(provider.DockerHub, resp)
	}

	var result struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, httpx.ErrorBodyLimit)).Decode(&result); err != nil {
		return "", fmt.Errorf("dockerhub: decode login response: %w", err)
	}

	if result.Token == "" {
		return "", fmt.Errorf("dockerhub: login response did not include a token: %w", provider.ErrInvalidResponse)
	}

	return result.Token, nil
}
