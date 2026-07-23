// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package provider

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by provider API clients, wrapped inside HTTPError.
var (
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("not found")
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrRateLimited     = errors.New("rate limited")
	ErrInvalidResponse = errors.New("invalid response")
)

// HTTPError describes a failed provider API call without leaking secrets or unbounded response bodies.
type HTTPError struct {
	Err        error  // Err classifies the provider response.
	Method     string // Method is the HTTP request method.
	URL        string // URL is sanitized for diagnostics.
	Body       string // Body is a bounded response excerpt.
	Provider   Type   // Provider identifies the API implementation.
	StatusCode int    // StatusCode is the HTTP response status.
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %s %s: status %d: %v", e.Provider, e.Method, e.URL, e.StatusCode, e.Err)
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}
