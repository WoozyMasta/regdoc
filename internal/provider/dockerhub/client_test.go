// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package dockerhub

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/woozymasta/regdoc/internal/httpx"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

func TestPublishSuccess(t *testing.T) {
	var loginBody, patchBody map[string]string

	var gotAuth, gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/users/login/":
			_ = json.NewDecoder(r.Body).Decode(&loginBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"jwt-token"}`))
		case "/v2/repositories/user/image/":
			gotAuth = r.Header.Get("Authorization")
			gotMethod = r.Method
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&patchBody)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.Client(), "user", "token")
	c.BaseURL = srv.URL

	tgt := target.Target{Registry: "index.docker.io", Repository: "user/image"}
	doc := provider.Document{Content: []byte("# Readme\n"), ShortDescription: "short"}

	if err := c.Publish(context.Background(), tgt, doc); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if loginBody["username"] != "user" || loginBody["password"] != "token" {
		t.Fatalf("unexpected login body: %+v", loginBody)
	}

	if gotMethod != http.MethodPatch || gotPath != "/v2/repositories/user/image/" {
		t.Fatalf("unexpected request: method=%s path=%s", gotMethod, gotPath)
	}

	if gotAuth != "JWT jwt-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}

	if patchBody["full_description"] != "# Readme\n" || patchBody["description"] != "short" {
		t.Fatalf("unexpected patch body: %+v", patchBody)
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
		{http.StatusInternalServerError, provider.ErrInvalidResponse},
	}

	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v2/users/login/" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"token":"jwt-token"}`))

				return
			}

			w.WriteHeader(tc.status)
		}))

		c := New(srv.Client(), "user", "token")
		c.BaseURL = srv.URL

		err := c.Publish(context.Background(), target.Target{Repository: "user/image"}, provider.Document{})
		if !errors.Is(err, tc.want) {
			t.Errorf("status %d: err = %v, want wrapping %v", tc.status, err, tc.want)
		}

		srv.Close()
	}
}

func TestListTagsPaginates(t *testing.T) {
	var gotAuth string

	var srv *httptest.Server

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v2/users/login/":
			_, _ = w.Write([]byte(`{"token":"jwt-token"}`))
		case "/v2/repositories/user/image/tags/":
			gotAuth = r.Header.Get("Authorization")
			if r.URL.RawQuery == "page_size=100" {
				_, _ = w.Write([]byte(`{"results":[{"name":"1.0.0"},{"name":"1.1.0"}],"next":"` + srv.URL + `/v2/repositories/user/image/tags/?page=2"}`))
			} else {
				_, _ = w.Write([]byte(`{"results":[{"name":"2.0.0"}],"next":null}`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.Client(), "user", "token")
	c.BaseURL = srv.URL

	tags, err := c.ListTags(context.Background(), target.Target{Repository: "user/image"})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}

	if gotAuth != "JWT jwt-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}

	want := []string{"1.0.0", "1.1.0", "2.0.0"}
	if !slices.Equal(tags, want) {
		t.Fatalf("ListTags = %v, want %v", tags, want)
	}
}

func TestListTagsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/users/login/" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"jwt-token"}`))

			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.Client(), "user", "token")
	c.BaseURL = srv.URL

	_, err := c.ListTags(context.Background(), target.Target{Repository: "user/image"})
	if !errors.Is(err, provider.ErrNotFound) {
		t.Fatalf("err = %v, want wrapping ErrNotFound", err)
	}
}

func TestListTagsResponseLargerThanErrorBodyLimit(t *testing.T) {
	padding := strings.Repeat("x", httpx.ErrorBodyLimit+1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v2/users/login/" {
			_, _ = w.Write([]byte(`{"token":"jwt-token"}`))
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]string{{"name": "1.0.0", "padding": padding}},
			"next":    nil,
		})
	}))
	defer srv.Close()

	c := New(srv.Client(), "user", "token")
	c.BaseURL = srv.URL

	tags, err := c.ListTags(context.Background(), target.Target{Repository: "user/image"})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if want := []string{"1.0.0"}; !slices.Equal(tags, want) {
		t.Fatalf("ListTags = %v, want %v", tags, want)
	}
}

func TestLoginMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := New(srv.Client(), "user", "token")
	c.BaseURL = srv.URL

	err := c.Publish(context.Background(), target.Target{Repository: "user/image"}, provider.Document{})
	if err == nil {
		t.Fatal("expected error for malformed login response")
	}
}

func TestPublishContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"jwt-token"}`))
	}))
	defer srv.Close()

	c := New(srv.Client(), "user", "token")
	c.BaseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Publish(ctx, target.Target{Repository: "user/image"}, provider.Document{})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}
