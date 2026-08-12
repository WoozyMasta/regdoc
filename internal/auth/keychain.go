// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package auth

import (
	"context"
	"fmt"

	"github.com/woozymasta/orascope"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/woozymasta/regdoc/internal/target"
)

// keychainCredentials resolves Docker-compatible credentials for the exact repository.
func keychainCredentials(tgt target.Target) (username, password string, ok bool, err error) {
	adapter, err := orascope.NewDefault()
	if err != nil {
		return "", "", false, fmt.Errorf("open repository-scoped credential adapter: %w", err)
	}

	ref := registry.Reference{
		Registry:   tgt.DistributionRegistry(),
		Repository: tgt.Repository,
	}
	ctx := auth.AppendRepositoryScope(context.Background(), ref, auth.ActionPull)
	credential, err := adapter.CredentialFunc(nil)(ctx, ref.Registry)
	if err != nil {
		return "", "", false, fmt.Errorf(
			"resolve docker keychain for %q: %w", tgt.Registry+"/"+tgt.Repository, err,
		)
	}

	if credential == auth.EmptyCredential {
		return "", "", false, nil
	}

	if credential.Username == "" && credential.Password == "" {
		return "", "", false, nil
	}

	return credential.Username, credential.Password, true, nil
}
