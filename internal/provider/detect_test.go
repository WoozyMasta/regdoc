// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/woozymasta/regdoc/internal/target"
)

const (
	quayHealthyBody = `{"data":{"services":{"auth":true,"database":true,"disk_space":true,
		"registry_gunicorn":true,"service_key":true,"web_gunicorn":true}},"status_code":200}`
	quayFalseServicesBody = `{"data":{"services":{"auth":false,"database":false,"disk_space":false,
		"registry_gunicorn":false,"service_key":false,"web_gunicorn":false}},"status_code":200}`
	quayMissingKeysBody = `{"data":{"services":{"database":true}},"status_code":200}`

	harborHealthyBody   = `{"status":"healthy","components":[{"name":"registry","status":"healthy"},{"name":"core","status":"healthy"}]}`
	harborUnhealthyBody = `{"status":"unhealthy","components":[{"name":"registry","status":"unhealthy"}]}`
	harborEmptyBody     = `{"status":"healthy","components":[]}`
	harborNoMatchBody   = `{"status":"healthy","components":[{"name":"something-else","status":"healthy"}]}`
)

// detectorFor creates a detector and target bound to srv.
func detectorFor(t *testing.T, srv *httptest.Server) (*Detector, target.Target) {
	t.Helper()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	return &Detector{Client: srv.Client(), Scheme: "http"}, target.Target{Registry: u.Host, Repository: "group/image"}
}

func TestDetectKnownHosts(t *testing.T) {
	cases := map[string]Type{
		"docker.io":            DockerHub,
		"index.docker.io":      DockerHub,
		"registry-1.docker.io": DockerHub,
		"quay.io":              Quay,
	}

	for host, want := range cases {
		d := &Detector{Client: &http.Client{Transport: rejectAnyRequest{t}}}
		tgt := target.Target{Registry: host, Repository: "group/image"}

		got, results, err := d.Detect(context.Background(), tgt)
		if err != nil {
			t.Fatalf("Detect(%q): %v", host, err)
		}

		if got != want {
			t.Errorf("Detect(%q) = %q, want %q", host, got, want)
		}

		if results != nil {
			t.Errorf("Detect(%q) performed network probes, want zero", host)
		}
	}
}

// rejectAnyRequest fails the test if a request is actually sent,
// used to assert that known-host resolution and explicit-provider paths perform zero network requests.
type rejectAnyRequest struct{ t *testing.T }

func (r rejectAnyRequest) RoundTrip(req *http.Request) (*http.Response, error) {
	r.t.Fatalf("unexpected network request: %s", req.URL)

	return nil, nil //nolint:nilnil // unreachable after Fatalf.
}

// failingTransport returns a deterministic transport error.
type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("network unavailable")
}

func TestDetectAggregatesTransportErrorsInProviderOrder(t *testing.T) {
	d := &Detector{Client: &http.Client{Transport: failingTransport{}}, Scheme: "https"}
	tgt := target.Target{Registry: "registry.example", Repository: "group/image"}

	_, results, err := d.Detect(context.Background(), tgt)
	if err == nil {
		t.Fatal("expected detection error")
	}

	if len(results) != 2 || results[0].Provider != Quay || results[1].Provider != Harbor {
		t.Fatalf("results = %#v", results)
	}

	message := err.Error()
	if !strings.Contains(message, "quay probe") || !strings.Contains(message, "harbor probe") {
		t.Fatalf("expected both probe errors, got %v", err)
	}
}

// jsonHandler returns a fixed JSON response handler.
func jsonHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestDetectQuaySchemas(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		matched bool
	}{
		{"healthy 200", http.StatusOK, quayHealthyBody, true},
		{"unhealthy 503 still matches", http.StatusServiceUnavailable, quayHealthyBody, true},
		{"false service values still match", http.StatusOK, quayFalseServicesBody, true},
		{"malformed json", http.StatusOK, "{not json", false},
		{"missing characteristic keys", http.StatusOK, quayMissingKeysBody, false},
		{"unexpected status", http.StatusNotFound, quayHealthyBody, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/health/instance":
					jsonHandler(tc.status, tc.body)(w, r)
				case "/api/v2.0/health":
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			d, tgt := detectorFor(t, srv)

			got, _, err := d.Detect(context.Background(), tgt)

			if tc.matched {
				if err != nil || got != Quay {
					t.Fatalf("expected Quay match, got provider=%q err=%v", got, err)
				}

				return
			}

			if err == nil {
				t.Fatalf("expected no match, got provider=%q", got)
			}
		})
	}
}

func TestDetectHarborSchemas(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		matched bool
	}{
		{"healthy", harborHealthyBody, true},
		{"unhealthy still matches", harborUnhealthyBody, true},
		{"empty components", harborEmptyBody, false},
		{"malformed json", "{not json", false},
		{"no characteristic component", harborNoMatchBody, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2.0/health":
					jsonHandler(http.StatusOK, tc.body)(w, r)
				case "/health/instance":
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			d, tgt := detectorFor(t, srv)

			got, _, err := d.Detect(context.Background(), tgt)

			if tc.matched {
				if err != nil || got != Harbor {
					t.Fatalf("expected Harbor match, got provider=%q err=%v", got, err)
				}

				return
			}

			if err == nil {
				t.Fatalf("expected no match, got provider=%q", got)
			}
		})
	}
}

func TestDetectBothMatchIsAmbiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health/instance":
			jsonHandler(http.StatusOK, quayHealthyBody)(w, r)
		case "/api/v2.0/health":
			jsonHandler(http.StatusOK, harborHealthyBody)(w, r)
		}
	}))
	defer srv.Close()

	d, tgt := detectorFor(t, srv)

	got, _, err := d.Detect(context.Background(), tgt)
	if err == nil {
		t.Fatalf("expected ambiguous error, got provider=%q", got)
	}

	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error message, got %v", err)
	}
}

func TestDetectBothNotFoundIsUndetected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d, tgt := detectorFor(t, srv)

	_, _, err := d.Detect(context.Background(), tgt)
	if err == nil {
		t.Fatal("expected could-not-be-detected error")
	}

	if !strings.Contains(err.Error(), "could not be detected") {
		t.Fatalf("expected could-not-be-detected error message, got %v", err)
	}

	if !strings.Contains(err.Error(), "-p quay") || !strings.Contains(err.Error(), "-p harbor") {
		t.Fatalf("expected error to suggest -p quay / -p harbor, got %v", err)
	}
}

func TestDetectOneMatchOneTimeoutUsesMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health/instance":
			jsonHandler(http.StatusOK, quayHealthyBody)(w, r)
		case "/api/v2.0/health":
			time.Sleep(DetectTimeout + 500*time.Millisecond)
		}
	}))
	defer srv.Close()

	d, tgt := detectorFor(t, srv)

	got, _, err := d.Detect(context.Background(), tgt)
	if err != nil || got != Quay {
		t.Fatalf("expected Quay match despite Harbor timeout, got provider=%q err=%v", got, err)
	}
}

func TestDetectRedirectIsInconclusive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer srv.Close()

	d, tgt := detectorFor(t, srv)

	_, _, err := d.Detect(context.Background(), tgt)
	if err == nil {
		t.Fatal("expected redirect to be treated as inconclusive (no match)")
	}
}

func TestDetectOversizedBodyDoesNotMatch(t *testing.T) {
	huge := strings.Repeat(" ", probeBodyLimit*2) + quayHealthyBody

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health/instance":
			jsonHandler(http.StatusOK, huge)(w, r)
		case "/api/v2.0/health":
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	d, tgt := detectorFor(t, srv)

	_, _, err := d.Detect(context.Background(), tgt)
	if err == nil {
		t.Fatal("expected oversized body to not match (truncated JSON)")
	}
}

func TestDetectContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d, tgt := detectorFor(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := d.Detect(ctx, tgt)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestDetectNoAuthHeadersOrCookies(t *testing.T) {
	var sawAuth, sawCookie bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			sawAuth = true
		}

		if len(r.Cookies()) > 0 {
			sawCookie = true
		}

		switch r.URL.Path {
		case "/health/instance":
			jsonHandler(http.StatusOK, quayHealthyBody)(w, r)
		case "/api/v2.0/health":
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	d, tgt := detectorFor(t, srv)

	if _, _, err := d.Detect(context.Background(), tgt); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if sawAuth || sawCookie {
		t.Fatal("probe request must not carry Authorization header or cookies")
	}
}

func TestKnownHost(t *testing.T) {
	if _, ok := KnownHost("example.com"); ok {
		t.Fatal("expected example.com to be unknown")
	}

	if pt, ok := KnownHost("quay.io"); !ok || pt != Quay {
		t.Fatalf("expected quay.io -> Quay, got %q ok=%v", pt, ok)
	}
}
