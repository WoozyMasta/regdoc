// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/woozymasta/regdoc/internal/auth"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/provider/distribution"
	"github.com/woozymasta/regdoc/internal/provider/dockerhub"
	"github.com/woozymasta/regdoc/internal/provider/harbor"
	"github.com/woozymasta/regdoc/internal/provider/quay"
	"github.com/woozymasta/regdoc/internal/target"
	registryauth "oras.land/oras-go/v2/registry/remote/auth"
)

// publishingClient combines a provider-specific description publisher
// with provider-independent OCI tag listing.
type publishingClient struct {
	provider.Publisher
	provider.TagLister
}

// fallbackTagLister preserves provider API tag listing
// when OCI Distribution credentials or capabilities are unavailable.
type fallbackTagLister struct {
	primary  provider.TagLister
	fallback provider.TagLister
}

// ListTags prefers OCI Distribution and falls back for capability/auth failures.
func (l fallbackTagLister) ListTags(ctx context.Context, tgt target.Target) ([]string, error) {
	tags, err := l.primary.ListTags(ctx, tgt)
	if err == nil {
		return tags, nil
	}
	if !errors.Is(err, provider.ErrUnauthorized) &&
		!errors.Is(err, provider.ErrForbidden) &&
		!errors.Is(err, provider.ErrNotFound) &&
		!errors.Is(err, provider.ErrInvalidResponse) {
		return nil, err
	}

	return l.fallback.ListTags(ctx, tgt)
}

// newPublisher is the only provider-to-client mapping.
// Credential resolution stays here so provider clients never read flags, environment, or keychains.
func newPublisher(
	httpClient *http.Client,
	providerType provider.Type,
	scheme string,
	explicit auth.Explicit,
	tgt target.Target,
) (provider.Publisher, error) {
	var (
		publisher         provider.Publisher
		credential        registryauth.Credential
		scopedCredentials bool
	)

	switch providerType {
	case provider.DockerHub:
		creds, err := auth.ResolveDockerHub(explicit, tgt)
		if err != nil {
			return nil, err
		}
		publisher = dockerhub.New(httpClient, creds.Username, creds.Token)
		credential = registryauth.Credential{Username: creds.Username, Password: creds.Token}
		scopedCredentials = explicit.Username == "" && explicit.Token == "" && explicit.Password == ""

	case provider.Quay:
		creds, err := auth.ResolveQuay(explicit)
		if err != nil {
			return nil, err
		}
		publisher = quay.New(httpClient, scheme, creds.Token)
		// A Quay metadata OAuth token does not imply OCI repository access.
		// ORAScope may still supply a separate Docker-compatible credential.
		scopedCredentials = true

	case provider.Harbor:
		creds, err := auth.ResolveHarbor(explicit, tgt)
		if err != nil {
			return nil, err
		}
		publisher = harbor.New(httpClient, scheme, creds.Username, creds.Password)
		credential = registryauth.Credential{Username: creds.Username, Password: creds.Password}
		scopedCredentials = explicit.Username == "" && explicit.Password == "" && explicit.Token == ""

	case provider.Unknown:
		return nil, fmt.Errorf("no provider resolved for %s", tgt.Hostname())

	default:
		return nil, fmt.Errorf("unsupported provider %q", providerType)
	}

	lister, err := distribution.NewTagLister(
		httpClient, providerType, credential, scheme == "http", scopedCredentials,
	)
	if err != nil {
		if fallback, ok := publisher.(provider.TagLister); ok {
			return publishingClient{Publisher: publisher, TagLister: fallback}, nil
		}
		return nil, err
	}

	fallback, ok := publisher.(provider.TagLister)
	if !ok {
		return publishingClient{Publisher: publisher, TagLister: lister}, nil
	}

	return publishingClient{
		Publisher: publisher,
		TagLister: fallbackTagLister{primary: lister, fallback: fallback},
	}, nil
}
