// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package httpx provides the shared HTTP client,
// retry policy and error mapping used by every provider API client.
package httpx

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"
)

// DefaultTimeout bounds one HTTP request when no timeout is configured.
const DefaultTimeout = 30 * time.Second

const (
	maxIdleConnsPerHost = 10                     // maxIdleConnsPerHost bounds retained idle connections per registry.
	idleConnTimeout     = 90 * time.Second       // idleConnTimeout closes unused persistent connections.
	dialTimeout         = 30 * time.Second       // dialTimeout bounds TCP connection establishment.
	maxRetries          = 3                      // maxRetries limits retries after the initial request.
	retryBaseDelay      = 200 * time.Millisecond // retryBaseDelay starts exponential retry backoff.
	retryMaxDelay       = 5 * time.Second        // retryMaxDelay caps retry backoff and Retry-After.
)

// Options configures NewClient.
type Options struct {
	UserAgent     string        // UserAgent is added when a request does not set one.
	Timeout       time.Duration // Timeout bounds each HTTP request; zero uses DefaultTimeout.
	TLSSkipVerify bool          // TLSSkipVerify disables certificate verification for this client only.
}

// retryingTransport adds a default User-Agent and bounded retries.
type retryingTransport struct {
	base      http.RoundTripper
	userAgent string
}

// NewClient builds an *http.Client configured per Options,
// with bounded retries for transient failures.
func NewClient(opts Options) *http.Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: dialTimeout, KeepAlive: dialTimeout}).DialContext,
		MaxIdleConns:        maxIdleConnsPerHost,
		MaxIdleConnsPerHost: maxIdleConnsPerHost,
		IdleConnTimeout:     idleConnTimeout,
	}

	if opts.TLSSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit, opt-in --tls-skip-verify.
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: &retryingTransport{base: transport, userAgent: opts.UserAgent},
	}
}

// RoundTrip implements http.RoundTripper.
func (t *retryingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.userAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", t.userAgent)
	}

	var lastResp *http.Response
	var lastErr error

	// The original request is used first; later attempts require a replayable body.
	for attempt := 0; ; attempt++ {
		reqAttempt := req

		if attempt > 0 {
			if err := waitForRetry(req, attempt, lastResp); err != nil {
				return nil, err
			}

			replay, err := replayBody(req)
			if err != nil {
				return lastResp, lastErr
			}

			reqAttempt = replay
		}

		resp, err := t.base.RoundTrip(reqAttempt) //nolint:bodyclose // body closed below or returned to caller.

		// Return non-retryable responses and the final retry result unchanged.
		if attempt >= maxRetries || !shouldRetry(resp, err) {
			return resp, err
		}

		// A discarded retry response must not leak its connection.
		if resp != nil {
			_ = resp.Body.Close()
		}

		lastResp, lastErr = resp, err
	}
}

// replayBody creates a fresh request body for a retry.
func replayBody(req *http.Request) (*http.Request, error) {
	if req.Body == nil {
		return req, nil
	}

	if req.GetBody == nil {
		return nil, errors.New("request body cannot be replayed for retry")
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}

	clone := req.Clone(req.Context())
	clone.Body = body
	return clone, nil
}

// waitForRetry waits for backoff or request cancellation.
func waitForRetry(req *http.Request, attempt int, resp *http.Response) error {
	timer := time.NewTimer(retryDelay(attempt, resp))
	defer timer.Stop()

	select {
	case <-req.Context().Done():
		return req.Context().Err()
	case <-timer.C:
		return nil
	}
}

// shouldRetry reports whether a response or error is retryable.
func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return retryNetworkError(err)
	}

	if resp == nil {
		return false
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true

	default:
		return false
	}
}

// retryNetworkError excludes cancellation and TLS verification failures.
func retryNetworkError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var verificationErr *tls.CertificateVerificationError
	if errors.As(err, &verificationErr) {
		return false
	}

	var netErr net.Error
	return errors.As(err, &netErr)
}

// retryDelay returns the bounded delay for a retry attempt.
func retryDelay(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if d, ok := retryAfter(resp.Header.Get("Retry-After")); ok {
			return d
		}
	}

	delay := retryBaseDelay * time.Duration(1<<uint(attempt-1)) //nolint:gosec // attempt is small and bounded by maxRetries.
	return min(delay, retryMaxDelay)
}

// retryAfter parses a Retry-After header value.
func retryAfter(value string) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}

	if secs, err := strconv.Atoi(value); err == nil {
		return capDelay(time.Duration(secs) * time.Second), true
	}

	if t, err := http.ParseTime(value); err == nil {
		d := max(time.Until(t), 0)

		return capDelay(d), true
	}

	return 0, false
}

// capDelay bounds a retry delay.
func capDelay(d time.Duration) time.Duration {
	return min(d, retryMaxDelay)
}
