// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package auth

import (
	"context"
	"fmt"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"

	"github.com/woozymasta/regdoc/internal/target"
)

// keychainCredentials delegates Docker config
// and credential-helper handling to the ORAS credential store.
// Do not parse Docker config or invoke helpers directly here.
func keychainCredentials(tgt target.Target) (username, password string, ok bool, err error) {
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return "", "", false, fmt.Errorf("open docker credential store: %w", err)
	}

	serverAddress := credentials.ServerAddressFromRegistry(tgt.Registry)
	if tgt.Registry == "index.docker.io" {
		serverAddress = credentials.ServerAddressFromRegistry("docker.io")
	}

	credential, err := store.Get(context.Background(), serverAddress)
	if err != nil {
		return "", "", false, fmt.Errorf("resolve docker keychain for %q: %w", tgt.Registry, err)
	}

	if credential == auth.EmptyCredential {
		return "", "", false, nil
	}

	if credential.Username == "" && credential.Password == "" {
		return "", "", false, nil
	}

	return credential.Username, credential.Password, true, nil
}
