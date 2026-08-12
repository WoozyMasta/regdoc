// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package distribution

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/woozymasta/regdoc/internal/httpx"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestListTagsUsesDistributionPaginationAndBasicAuth(t *testing.T) {
	var requests atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if user != "user" || password != "password" {
			t.Errorf("basic auth = %q/%q", user, password)
		}

		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("last") == "" {
			w.Header().Set("Link", "<"+srv.URL+"/v2/team/image/tags/list?last=1.1.0>; rel=\"next\"")
			_, _ = fmt.Fprintf(
				w,
				`{"name":"team/image","tags":["1.0.0","1.1.0"],"padding":%q}`,
				strings.Repeat("x", httpx.ErrorBodyLimit+1),
			)
			return
		}
		_, _ = w.Write([]byte(`{"name":"team/image","tags":["2.0.0"]}`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	lister, err := NewTagLister(
		srv.Client(), provider.Harbor,
		auth.Credential{Username: "user", Password: "password"}, true, false,
	)
	if err != nil {
		t.Fatalf("NewTagLister: %v", err)
	}

	tags, err := lister.ListTags(
		context.Background(), target.Target{Registry: u.Host, Repository: "team/image"},
	)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if want := []string{"1.0.0", "1.1.0", "2.0.0"}; !slices.Equal(tags, want) {
		t.Fatalf("ListTags = %v, want %v", tags, want)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("authenticated requests = %d, want 2", got)
	}
}

func TestListTagsUsesRepositoryScopedCredentials(t *testing.T) {
	var gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		gotUser = user
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"team/image","tags":["1.0.0"]}`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	scopedKey := u.Host + "/team/image"
	authValue := base64.StdEncoding.EncodeToString([]byte("scoped:secret"))
	config := fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, scopedKey, authValue)
	encodedConfig := base64.StdEncoding.EncodeToString([]byte(config))
	t.Setenv("DOCKER_AUTH_CONFIG", config)
	t.Setenv("DOCKER_AUTH_CONFIG_BASE64", encodedConfig)
	t.Setenv("DOCKER_AUTH_CONFIG_ENCODED", encodedConfig)

	lister, err := NewTagLister(
		srv.Client(), provider.Harbor,
		auth.Credential{Username: "host", Password: "fallback"}, true, true,
	)
	if err != nil {
		t.Fatalf("NewTagLister: %v", err)
	}
	_, err = lister.ListTags(
		context.Background(), target.Target{Registry: u.Host, Repository: "team/image"},
	)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if gotUser != "scoped" {
		t.Fatalf("username = %q, want repository-scoped credential", gotUser)
	}
}

func TestListTagsRejectsRepeatedPage(t *testing.T) {
	var requests atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", "<"+srv.URL+"/v2/team/image/tags/list?next=again>; rel=\"next\"")
		_, _ = w.Write([]byte(`{"name":"team/image","tags":["1.0.0"]}`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	lister, err := NewTagLister(srv.Client(), provider.Harbor, auth.EmptyCredential, true, false)
	if err != nil {
		t.Fatalf("NewTagLister: %v", err)
	}
	_, err = lister.ListTags(
		context.Background(), target.Target{Registry: u.Host, Repository: "team/image"},
	)
	if !errors.Is(err, provider.ErrInvalidResponse) {
		t.Fatalf("err = %v, want wrapping ErrInvalidResponse", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestListTagsRejectsCrossOriginPagination(t *testing.T) {
	var externalCalled atomic.Bool
	external := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		externalCalled.Store(true)
	}))
	defer external.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", "<"+external.URL+"/steal>; rel=\"next\"")
		_, _ = w.Write([]byte(`{"name":"team/image","tags":["1.0.0"]}`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	lister, err := NewTagLister(srv.Client(), provider.Harbor, auth.EmptyCredential, true, false)
	if err != nil {
		t.Fatalf("NewTagLister: %v", err)
	}
	_, err = lister.ListTags(
		context.Background(), target.Target{Registry: u.Host, Repository: "team/image"},
	)
	if !errors.Is(err, provider.ErrInvalidResponse) ||
		!strings.Contains(err.Error(), "not the same origin") {
		t.Fatalf("err = %v, want same-origin rejection", err)
	}
	if externalCalled.Load() {
		t.Fatal("cross-origin server received a request")
	}
}

func TestListTagsClassifiesRegistryStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"code":"NAME_UNKNOWN","message":"missing"}]}`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	lister, err := NewTagLister(srv.Client(), provider.Quay, auth.EmptyCredential, true, false)
	if err != nil {
		t.Fatalf("NewTagLister: %v", err)
	}
	_, err = lister.ListTags(
		context.Background(), target.Target{Registry: u.Host, Repository: "team/image"},
	)
	if !errors.Is(err, provider.ErrNotFound) {
		t.Fatalf("err = %v, want wrapping ErrNotFound", err)
	}
}
