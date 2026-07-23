// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package httpx

import (
	"io"
	"net/http"
	"net/url"

	"github.com/woozymasta/regdoc/internal/provider"
)

// ErrorBodyLimit caps how much of an error response body is read and embedded in the resulting error.
// ErrorBodyLimit bounds response text retained in an error.
const ErrorBodyLimit = 4 * 1024

// NewHTTPError builds a provider.HTTPError from a non-2xx response,
// reading and closing its body (bounded by ErrorBodyLimit)
// and classifying it against the provider sentinel errors.
func NewHTTPError(providerType provider.Type, resp *http.Response) *provider.HTTPError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, ErrorBodyLimit))
	_ = resp.Body.Close()

	return &provider.HTTPError{
		Provider:   providerType,
		Method:     resp.Request.Method,
		URL:        SanitizeURL(resp.Request.URL),
		StatusCode: resp.StatusCode,
		Body:       string(body),
		Err:        classify(resp.StatusCode),
	}
}

// SanitizeURL renders u without credentials or query parameters,
// so error messages and debug output never leak secrets passed via URL.
func SanitizeURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	sanitized := *u
	sanitized.User = nil
	sanitized.RawQuery = ""
	sanitized.Fragment = ""

	return sanitized.String()
}

// IsSuccess reports whether statusCode is any 2xx response.
func IsSuccess(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

// classify maps an HTTP status to the provider error contract.
func classify(statusCode int) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return provider.ErrUnauthorized

	case http.StatusForbidden:
		return provider.ErrForbidden

	case http.StatusNotFound:
		return provider.ErrNotFound

	case http.StatusRequestEntityTooLarge:
		return provider.ErrPayloadTooLarge

	case http.StatusTooManyRequests:
		return provider.ErrRateLimited

	default:
		return provider.ErrInvalidResponse
	}
}
