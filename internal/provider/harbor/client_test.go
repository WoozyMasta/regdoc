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
	"slices"
	"strings"
	"testing"

	"github.com/woozymasta/regdoc/internal/httpx"
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

func TestListTagsFlattensArtifactTags(t *testing.T) {
	var gotUser, gotPass, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"tags":[{"name":"1.0.0"},{"name":"stable"}]},
			{"tags":[{"name":"1.1.0"}]},
			{"tags":[]}
		]`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := New(srv.Client(), "http", "harbor-user", "harbor-pass")

	tags, err := c.ListTags(context.Background(), target.Target{Registry: u.Host, Repository: "project/team/image"})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}

	if gotPath != "/api/v2.0/projects/project/repositories/team%2Fimage/artifacts" {
		t.Fatalf("path = %q", gotPath)
	}

	if gotUser != "harbor-user" || gotPass != "harbor-pass" {
		t.Fatalf("basic auth = %q/%q", gotUser, gotPass)
	}

	want := []string{"1.0.0", "stable", "1.1.0"}
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
	c := New(srv.Client(), "http", "harbor-user", "harbor-pass")

	_, err := c.ListTags(context.Background(), target.Target{Registry: u.Host, Repository: "project/image"})
	if !errors.Is(err, provider.ErrNotFound) {
		t.Fatalf("err = %v, want wrapping ErrNotFound", err)
	}
}

func TestListTagsResponseLargerThanErrorBodyLimit(t *testing.T) {
	padding := strings.Repeat("x", httpx.ErrorBodyLimit+1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"tags":    []map[string]string{{"name": "1.0.0"}},
			"padding": padding,
		}})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := New(srv.Client(), "http", "harbor-user", "harbor-pass")

	tags, err := c.ListTags(context.Background(), target.Target{Registry: u.Host, Repository: "project/image"})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if want := []string{"1.0.0"}; !slices.Equal(tags, want) {
		t.Fatalf("ListTags = %v, want %v", tags, want)
	}
}

func TestListTagsInvalidRepository(t *testing.T) {
	c := New(http.DefaultClient, "http", "u", "p")

	_, err := c.ListTags(
		context.Background(),
		target.Target{Registry: "harbor.example.com", Repository: "no-project-segment"},
	)
	if err == nil {
		t.Fatal("expected error for repository without a project segment")
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
