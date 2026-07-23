// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package harbor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

func TestPublishSuccessNestedRepository(t *testing.T) {
	var gotUser, gotPass, gotMethod, gotPath string

	var body map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := New(srv.Client(), "http", "harbor-user", "harbor-pass")

	tgt := target.Target{Registry: u.Host, Repository: "project/team/image"}
	doc := provider.Document{Content: []byte("# Readme\n")}

	if err := c.Publish(context.Background(), tgt, doc); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("method = %q, want PUT", gotMethod)
	}

	if gotPath != "/api/v2.0/projects/project/repositories/team%2Fimage" {
		t.Fatalf("path = %q", gotPath)
	}

	if gotUser != "harbor-user" || gotPass != "harbor-pass" {
		t.Fatalf("basic auth = %q/%q", gotUser, gotPass)
	}

	if body["description"] != "# Readme\n" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestPublishInvalidRepository(t *testing.T) {
	c := New(http.DefaultClient, "http", "u", "p")

	err := c.Publish(
		context.Background(),
		target.Target{Registry: "harbor.example.com", Repository: "no-project-segment"},
		provider.Document{},
	)
	if err == nil {
		t.Fatal("expected error for repository without a project segment")
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
		{http.StatusServiceUnavailable, provider.ErrInvalidResponse},
	}

	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}))

		u, _ := url.Parse(srv.URL)
		c := New(srv.Client(), "http", "u", "p")

		err := c.Publish(
			context.Background(),
			target.Target{Registry: u.Host, Repository: "project/image"},
			provider.Document{},
		)
		if !errors.Is(err, tc.want) {
			t.Errorf("status %d: err = %v, want wrapping %v", tc.status, err, tc.want)
		}

		srv.Close()
	}
}
