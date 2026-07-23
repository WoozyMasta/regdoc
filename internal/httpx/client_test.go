// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package httpx

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientSetsUserAgent(t *testing.T) {
	var gotUA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(Options{UserAgent: "regdoc/test"})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if gotUA != "regdoc/test" {
		t.Fatalf("User-Agent = %q, want %q", gotUA, "regdoc/test")
	}
}

func TestNewClientRetriesOn503ThenSucceeds(t *testing.T) {
	var attempts int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(Options{})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("final status = %d, want 200", resp.StatusCode)
	}

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestNewClientDoesNotRetry404(t *testing.T) {
	var attempts int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(Options{})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (no retry on 404)", attempts)
	}
}

func TestNewClientHonorsRetryAfter(t *testing.T) {
	var attempts int

	var firstAt, secondAt time.Time

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			firstAt = time.Now()
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}

		secondAt = time.Now()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(Options{})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if secondAt.Sub(firstAt) < 900*time.Millisecond {
		t.Fatalf("retry happened too early: %v", secondAt.Sub(firstAt))
	}
}

func TestNewClientTLSSkipVerifyScopedToTransport(t *testing.T) {
	insecure := NewClient(Options{TLSSkipVerify: true})
	secure := NewClient(Options{})

	insecureRT, ok := insecure.Transport.(*retryingTransport)
	if !ok {
		t.Fatalf("expected *retryingTransport, got %T", insecure.Transport)
	}

	base, ok := insecureRT.base.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport base, got %T", insecureRT.base)
	}

	if base.TLSClientConfig == nil || !base.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify on the scoped transport")
	}

	secureRT, _ := secure.Transport.(*retryingTransport)
	secureBase, _ := secureRT.base.(*http.Transport)

	if secureBase.TLSClientConfig != nil && secureBase.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLSSkipVerify leaked into an unrelated client")
	}
}

func TestNewClientDefaultTimeout(t *testing.T) {
	client := NewClient(Options{})
	if client.Timeout != DefaultTimeout {
		t.Fatalf("Timeout = %v, want %v", client.Timeout, DefaultTimeout)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	d, ok := retryAfter("2")
	if !ok || d != 2*time.Second {
		t.Fatalf("retryAfter(2) = %v, %v", d, ok)
	}

	if _, ok := retryAfter(""); ok {
		t.Fatal("expected no value for empty Retry-After")
	}

	if _, ok := retryAfter("not-a-number-or-date"); ok {
		t.Fatal("expected no value for garbage Retry-After")
	}
}

func TestNewClientNoRetryWithoutReplayableBody(t *testing.T) {
	var attempts int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := NewClient(Options{})

	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write([]byte("body"))
		_ = pw.Close()
	}()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, pr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	// A streaming body (no GetBody) must not be retried.
	req.GetBody = nil

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (body not replayable)", attempts)
	}
}

func TestShouldRetryNetworkErrors(t *testing.T) {
	connectionReset := &net.OpError{Err: errors.New("connection reset")}
	if !shouldRetry(nil, connectionReset) {
		t.Fatal("expected network error to be retried")
	}

	if shouldRetry(nil, context.Canceled) {
		t.Fatal("canceled context must not be retried")
	}
}
