// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/woozymasta/regdoc/internal/auth"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

func TestNewPublisherFallsBackToQuayAPIForMetadataOnlyToken(t *testing.T) {
	var distributionAuth, metadataAuth string
	var distributionRequests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/team/image/tags/list":
			distributionRequests++
			distributionAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errors":[{"code":"UNAUTHORIZED","message":"denied"}]}`))
		case "/api/v1/repository/team/image/tag/":
			metadataAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":[{"name":"1.0.0"}],"has_additional":false}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	tgt := target.Target{Registry: u.Host, Repository: "team/image"}
	pub, err := newPublisher(
		srv.Client(), provider.Quay, "http", auth.Explicit{Token: "quay-token"}, tgt,
	)
	if err != nil {
		t.Fatalf("newPublisher: %v", err)
	}

	lister, ok := pub.(provider.TagLister)
	if !ok {
		t.Fatal("publisher does not implement TagLister")
	}
	tags, err := lister.ListTags(context.Background(), tgt)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 1 || tags[0] != "1.0.0" {
		t.Fatalf("tags = %v", tags)
	}
	if distributionAuth != "" {
		t.Fatalf("Quay API token leaked to Distribution API: %q", distributionAuth)
	}
	if distributionRequests != 0 {
		t.Fatalf("Distribution API requests = %d, want 0 without registry credentials", distributionRequests)
	}
	if metadataAuth != "Bearer quay-token" {
		t.Fatalf("metadata Authorization = %q", metadataAuth)
	}
}
