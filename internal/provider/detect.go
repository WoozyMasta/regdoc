// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/woozymasta/regdoc/internal/target"
)

// DetectTimeout bounds unauthenticated provider probing.
const DetectTimeout = 3 * time.Second

// probeBodyLimit bounds data read from an unauthenticated probe response.
const probeBodyLimit = 64 * 1024

var knownHosts = map[string]Type{
	"docker.io":            DockerHub,
	"index.docker.io":      DockerHub,
	"registry-1.docker.io": DockerHub,
	"quay.io":              Quay,
}

var harborComponents = map[string]struct{}{
	"registry":    {},
	"registryctl": {},
	"portal":      {},
	"core":        {},
	"jobservice":  {},
	"database":    {},
	"redis":       {},
}

// ProbeResult is the typed outcome of a single unauthenticated provider probe,
// distinguishing a schema mismatch from a transport failure.
type ProbeResult struct {
	Err        error // Err is the transport failure, if any.
	Provider   Type  // Provider is the probed registry implementation.
	StatusCode int   // StatusCode is zero when no response was received.
	Matched    bool  // Matched reports whether the provider schema was recognized.
}

// Detector performs unauthenticated custom-registry provider detection.
type Detector struct {
	Client    *http.Client  // Client executes unauthenticated probes.
	Scheme    string        // Scheme is https unless plain HTTP is explicitly selected.
	UserAgent string        // UserAgent identifies probe requests.
	Timeout   time.Duration // Timeout bounds both probes; zero uses DetectTimeout.
}

// KnownHost returns the provider mapped to hostname, if any.
func KnownHost(hostname string) (Type, bool) {
	t, ok := knownHosts[hostname]
	return t, ok
}

// Detect resolves the provider for a custom (not well-known) hostname
// by probing Quay and Harbor health endpoints concurrently.
// It never touches auth or constructs a Publisher.
func (d *Detector) Detect(ctx context.Context, tgt target.Target) (Type, []ProbeResult, error) {
	if t, ok := KnownHost(tgt.Hostname()); ok {
		return t, nil, nil // Public registries do not need network probing.
	}

	timeout := d.Timeout
	if timeout <= 0 {
		timeout = DetectTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run independent provider probes concurrently under the shared deadline.
	results := make(chan ProbeResult, 2)

	go func() { results <- d.probeQuay(ctx, tgt) }()
	go func() { results <- d.probeHarbor(ctx, tgt) }()

	// Normalize result order so diagnostics and tests are deterministic.
	byProvider := make(map[Type]ProbeResult, 2)
	for range 2 {
		result := <-results
		byProvider[result.Provider] = result
	}

	all := []ProbeResult{byProvider[Quay], byProvider[Harbor]}
	matched := make([]ProbeResult, 0, len(all))
	transportErrs := make([]error, 0, len(all))

	for _, r := range all {
		if r.Matched {
			matched = append(matched, r)
		} else if r.Err != nil {
			transportErrs = append(transportErrs, fmt.Errorf("%s probe: %w", r.Provider, r.Err))
		}
	}

	// A single schema match is authoritative; conflicting or failed probes need an explicit provider selection.
	switch len(matched) {
	case 1:
		return matched[0].Provider, all, nil

	case 2:
		return Unknown, all, fmt.Errorf(
			"provider for %s is ambiguous (matched both quay and harbor); specify -p quay or -p harbor", tgt.Hostname())

	default:
		if len(transportErrs) > 0 {
			return Unknown, all, fmt.Errorf(
				"provider detection for %s failed: %w; specify -p quay or -p harbor",
				tgt.Hostname(),
				errors.Join(transportErrs...),
			)
		}

		return Unknown, all, fmt.Errorf(
			"provider for %s could not be detected; specify -p quay or -p harbor", tgt.Hostname())
	}
}

// newRequest creates an unauthenticated provider probe request.
func (d *Detector) newRequest(ctx context.Context, tgt target.Target, path string) (*http.Request, error) {
	scheme := d.Scheme
	if scheme == "" {
		scheme = "https"
	}

	url := scheme + "://" + tgt.Registry + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")

	if d.UserAgent != "" {
		req.Header.Set("User-Agent", d.UserAgent)
	}

	return req, nil
}

// do disables redirects: redirected health endpoints are not provider evidence.
func (d *Detector) do(req *http.Request) (*http.Response, error) {
	client := *d.Client
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return client.Do(req) //nolint:bodyclose // caller closes the response body.
}

// probeQuay matches Quay-specific health response fields.
func (d *Detector) probeQuay(ctx context.Context, tgt target.Target) ProbeResult {
	result := ProbeResult{Provider: Quay}

	req, err := d.newRequest(ctx, tgt, "/health/instance")
	if err != nil {
		result.Err = err
		return result
	}

	resp, err := d.do(req)
	if err != nil {
		result.Err = err
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	result.StatusCode = resp.StatusCode

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		return result
	}

	var body struct {
		Data *struct {
			Services map[string]bool `json:"services"`
		} `json:"data"`
		StatusCode *int `json:"status_code"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, probeBodyLimit)).Decode(&body); err != nil {
		return result
	}

	if body.Data == nil || body.Data.Services == nil || body.StatusCode == nil {
		return result
	}

	required := []string{"registry_gunicorn", "web_gunicorn", "service_key"}
	for _, key := range required {
		if _, ok := body.Data.Services[key]; !ok {
			return result
		}
	}

	result.Matched = true
	return result
}

// probeHarbor matches Harbor-specific health response fields.
func (d *Detector) probeHarbor(ctx context.Context, tgt target.Target) ProbeResult {
	result := ProbeResult{Provider: Harbor}

	req, err := d.newRequest(ctx, tgt, "/api/v2.0/health")
	if err != nil {
		result.Err = err
		return result
	}

	resp, err := d.do(req)
	if err != nil {
		result.Err = err
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	result.StatusCode = resp.StatusCode

	var body struct {
		Status     *string `json:"status"`
		Components []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"components"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, probeBodyLimit)).Decode(&body); err != nil {
		return result
	}

	if body.Status == nil || len(body.Components) == 0 {
		return result
	}

	for _, c := range body.Components {
		if _, ok := harborComponents[c.Name]; ok {
			result.Matched = true

			break
		}
	}

	return result
}
