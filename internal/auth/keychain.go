// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package auth

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/woozymasta/regdoc/internal/target"
)

// keychainCredentials delegates Docker config and credential-helper handling to authn.DefaultKeychain.
// Do not parse Docker config or invoke helpers here.
func keychainCredentials(tgt target.Target) (username, password string, ok bool, err error) {
	reg, err := name.NewRegistry(tgt.Registry)
	if err != nil {
		return "", "", false, fmt.Errorf("resolve registry %q for keychain lookup: %w", tgt.Registry, err)
	}

	authenticator, err := authn.DefaultKeychain.Resolve(reg)
	if err != nil {
		return "", "", false, fmt.Errorf("resolve docker keychain for %q: %w", tgt.Registry, err)
	}

	if authenticator == authn.Anonymous {
		return "", "", false, nil
	}

	cfg, err := authenticator.Authorization()
	if err != nil {
		return "", "", false, fmt.Errorf("read keychain credentials for %q: %w", tgt.Registry, err)
	}

	if cfg.Username == "" && cfg.Password == "" {
		return "", "", false, nil
	}

	return cfg.Username, cfg.Password, true, nil
}
