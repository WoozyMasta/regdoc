// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"fmt"
	"net/http"

	"github.com/woozymasta/regdoc/internal/auth"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/provider/dockerhub"
	"github.com/woozymasta/regdoc/internal/provider/harbor"
	"github.com/woozymasta/regdoc/internal/provider/quay"
	"github.com/woozymasta/regdoc/internal/target"
)

// newPublisher is the only provider-to-client mapping.
// Credential resolution stays here so provider clients never read flags, environment, or keychains.
func newPublisher(
	httpClient *http.Client,
	providerType provider.Type,
	scheme string,
	explicit auth.Explicit,
	tgt target.Target,
) (provider.Publisher, error) {
	switch providerType {
	case provider.DockerHub:
		creds, err := auth.ResolveDockerHub(explicit, tgt)
		if err != nil {
			return nil, err
		}
		return dockerhub.New(httpClient, creds.Username, creds.Token), nil

	case provider.Quay:
		creds, err := auth.ResolveQuay(explicit)
		if err != nil {
			return nil, err
		}
		return quay.New(httpClient, scheme, creds.Token), nil

	case provider.Harbor:
		creds, err := auth.ResolveHarbor(explicit, tgt)
		if err != nil {
			return nil, err
		}
		return harbor.New(httpClient, scheme, creds.Username, creds.Password), nil

	case provider.Unknown:
		return nil, fmt.Errorf("no provider resolved for %s", tgt.Hostname())

	default:
		return nil, fmt.Errorf("unsupported provider %q", providerType)
	}
}
