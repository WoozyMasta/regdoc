// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package quay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

func TestPublishSuccess(t *testing.T) {
	var gotAuth, gotMethod, gotPath string

	var body map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)

	c := New(srv.Client(), "http", "quay-token")

	tgt := target.Target{Registry: u.Host, Repository: "group/image"}
	doc := provider.Document{Content: []byte("# Readme\n")}

	if err := c.Publish(context.Background(), tgt, doc); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if gotMethod != http.MethodPut || gotPath != "/api/v1/repository/group/image" {
		t.Fatalf("unexpected request: method=%s path=%s", gotMethod, gotPath)
	}

	if gotAuth != "Bearer quay-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}

	if body["description"] != "# Readme\n" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestListTagsPaginates(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"tags":[{"name":"1.0.0"},{"name":"1.1.0"}],"page":1,"has_additional":true}`))
		} else {
			_, _ = w.Write([]byte(`{"tags":[{"name":"2.0.0"}],"page":2,"has_additional":false}`))
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := New(srv.Client(), "http", "quay-token")

	tags, err := c.ListTags(context.Background(), target.Target{Registry: u.Host, Repository: "group/image"})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}

	if gotAuth != "Bearer quay-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}

	want := []string{"1.0.0", "1.1.0", "2.0.0"}
	if !slices.Equal(tags, want) {
		t.Fatalf("ListTags = %v, want %v", tags, want)
	}
}

func TestListTagsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := New(srv.Client(), "http", "quay-token")

	_, err := c.ListTags(context.Background(), target.Target{Registry: u.Host, Repository: "group/image"})
	if !errors.Is(err, provider.ErrNotFound) {
		t.Fatalf("err = %v, want wrapping ErrNotFound", err)
	}
}

func TestPublishStatusErrors(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, provider.ErrUnauthorized},
		{http.StatusForbidden, provider.ErrForbidden},
		{http.StatusNotFound, provider.ErrNotFound},
		{http.StatusRequestEntityTooLarge, provider.ErrPayloadTooLarge},
		{http.StatusTooManyRequests, provider.ErrRateLimited},
		{http.StatusBadGateway, provider.ErrInvalidResponse},
	}

	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}))

		u, _ := url.Parse(srv.URL)
		c := New(&http.Client{Transport: http.DefaultTransport}, "http", "token")

		err := c.Publish(
			context.Background(),
			target.Target{Registry: u.Host, Repository: "group/image"},
			provider.Document{},
		)
		if !errors.Is(err, tc.want) {
			t.Errorf("status %d: err = %v, want wrapping %v", tc.status, err, tc.want)
		}

		srv.Close()
	}
}

func TestPublishContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := New(srv.Client(), "http", "token")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Publish(ctx, target.Target{Registry: u.Host, Repository: "group/image"}, provider.Document{})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}
