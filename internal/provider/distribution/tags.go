// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package distribution implements provider-independent OCI Distribution API operations.
package distribution

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/woozymasta/orascope"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"

	"github.com/woozymasta/regdoc/internal/httpx"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

const tagListPageSize = 100

// TagLister lists repository tags through the OCI Distribution API.
type TagLister struct {
	httpClient *http.Client
	adapter    *orascope.Adapter
	credential auth.Credential
	provider   provider.Type
	plainHTTP  bool
}

// NewTagLister creates an OCI tag lister. When scopedCredentials is true,
// Docker-compatible repository-scoped credentials override the host fallback.
func NewTagLister(
	httpClient *http.Client,
	providerType provider.Type,
	credential auth.Credential,
	plainHTTP bool,
	scopedCredentials bool,
) (*TagLister, error) {
	lister := &TagLister{
		httpClient: httpClient,
		credential: credential,
		provider:   providerType,
		plainHTTP:  plainHTTP,
	}

	if scopedCredentials {
		adapter, err := orascope.NewDefault()
		if err != nil {
			return nil, fmt.Errorf("load repository-scoped registry credentials: %w", err)
		}
		lister.adapter = adapter
	}

	return lister, nil
}

// ListTags returns every tag from the repository's OCI Distribution endpoint.
func (l *TagLister) ListTags(ctx context.Context, tgt target.Target) ([]string, error) {
	registryHost := tgt.DistributionRegistry()
	repo, err := remote.NewRepository(registryHost + "/" + tgt.Repository)
	if err != nil {
		return nil, fmt.Errorf("create distribution repository: %w", err)
	}

	if l.adapter != nil && l.credential == auth.EmptyCredential {
		scopeCtx := auth.AppendRepositoryScope(ctx, repo.Reference, auth.ActionPull)
		credential, resolveErr := l.adapter.CredentialFunc(nil)(scopeCtx, registryHost)
		if resolveErr != nil {
			return nil, fmt.Errorf("resolve distribution credential: %w", resolveErr)
		}

		if credential == auth.EmptyCredential {
			return nil, fmt.Errorf(
				"no OCI Distribution credential for %s: %w",
				registryHost+"/"+tgt.Repository, provider.ErrUnauthorized,
			)
		}
	}

	client := &auth.Client{
		Client:     l.httpClient,
		Cache:      auth.NewCache(),
		Credential: auth.StaticCredential(registryHost, l.credential),
	}
	if l.adapter != nil {
		client, err = l.adapter.WrapAuthClient(client)
		if err != nil {
			return nil, fmt.Errorf("wrap distribution auth client: %w", err)
		}
	}

	repo.Client = client
	repo.PlainHTTP = l.plainHTTP
	repo.TagListPageSize = tagListPageSize

	var tags []string
	seenPages := make(map[string]struct{})
	err = repo.Tags(ctx, "", func(page []string) error {
		pageKey := strings.Join(page, "\x00")
		if _, seen := seenPages[pageKey]; seen {
			return fmt.Errorf("repeated tag page: %w", provider.ErrInvalidResponse)
		}
		seenPages[pageKey] = struct{}{}
		tags = append(tags, page...)

		return nil
	})
	if err != nil {
		return nil, classifyError(l.provider, err)
	}

	return tags, nil
}

// classifyError preserves the provider error contract for ORAS responses.
func classifyError(providerType provider.Type, err error) error {
	var response *errcode.ErrorResponse
	if errors.As(err, &response) {
		return &provider.HTTPError{
			Provider:   providerType,
			Method:     response.Method,
			URL:        httpx.SanitizeURL(response.URL),
			StatusCode: response.StatusCode,
			Body:       response.Errors.Error(),
			Err:        classifyStatus(response.StatusCode),
		}
	}

	var networkError net.Error
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.As(err, &networkError) ||
		errors.Is(err, orascope.ErrAmbiguousRepositoryCredentials) {
		return err
	}

	return fmt.Errorf("%v: %w", err, provider.ErrInvalidResponse)
}

// classifyStatus maps registry status codes to provider sentinel errors.
func classifyStatus(statusCode int) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return provider.ErrUnauthorized
	case http.StatusForbidden:
		return provider.ErrForbidden
	case http.StatusNotFound:
		return provider.ErrNotFound
	case http.StatusTooManyRequests:
		return provider.ErrRateLimited
	default:
		return provider.ErrInvalidResponse
	}
}
