// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package auth resolves credentials for the selected registry provider.
package auth

import (
	"errors"
	"fmt"

	"github.com/woozymasta/regdoc/internal/target"
)

// Explicit contains credentials supplied through flags or environment variables.
type Explicit struct {
	Username string // Username is the registry account name.
	Password string // Password is the registry password.
	Token    string // Token is an API token when the provider supports it.
}

// DockerHubCredentials authenticates against the Docker Hub metadata API.
type DockerHubCredentials struct {
	Username string // Username is the Docker Hub account name.
	Token    string // Token is the Docker Hub password or access token.
}

// String redacts Token so credentials never leak through %v/%s logging.
func (c DockerHubCredentials) String() string {
	return fmt.Sprintf(
		"DockerHubCredentials{Username: %q, Token: %s}",
		c.Username,
		redact(c.Token),
	)
}

// QuayCredentials authenticates against the Quay API via an OAuth bearer
// token.
type QuayCredentials struct {
	Token string // Token is the Quay OAuth bearer token.
}

// String redacts Token so credentials never leak through %v/%s logging.
func (c QuayCredentials) String() string {
	return fmt.Sprintf(
		"QuayCredentials{Token: %s}",
		redact(c.Token),
	)
}

// HarborCredentials authenticates against the Harbor API via HTTP Basic
// auth.
type HarborCredentials struct {
	Username string // Username is the registry account name.
	Password string // Password is the registry password.
}

// String redacts Password so credentials never leak through %v/%s logging.
func (c HarborCredentials) String() string {
	return fmt.Sprintf(
		"HarborCredentials{Username: %q, Password: %s}",
		c.Username,
		redact(c.Password),
	)
}

// ResolveDockerHub prefers explicit credentials, then the Docker keychain.
func ResolveDockerHub(explicit Explicit, tgt target.Target) (DockerHubCredentials, error) {
	username := explicit.Username
	token := firstNonEmpty(explicit.Token, explicit.Password)

	if username != "" && token != "" {
		return DockerHubCredentials{Username: username, Token: token}, nil
	}

	kcUser, kcPass, ok, err := keychainCredentials(tgt)
	if err != nil {
		return DockerHubCredentials{}, err
	}

	if ok {
		username = firstNonEmpty(username, kcUser)
		token = firstNonEmpty(token, kcPass)
	}

	if username == "" || token == "" {
		return DockerHubCredentials{}, fmt.Errorf(
			"no Docker Hub credentials found for %s",
			tgt.Hostname(),
		)
	}

	return DockerHubCredentials{Username: username, Token: token}, nil
}

// ResolveQuay accepts only an explicit OAuth token;
// Docker credentials are not valid Quay tokens.
func ResolveQuay(explicit Explicit) (QuayCredentials, error) {
	token := explicit.Token
	if token == "" {
		return QuayCredentials{}, errors.New("no Quay OAuth token found; set --token or REGDOC_TOKEN")
	}

	return QuayCredentials{Token: token}, nil
}

// ResolveHarbor prefers explicit credentials, then the Docker keychain.
func ResolveHarbor(explicit Explicit, tgt target.Target) (HarborCredentials, error) {
	username := explicit.Username
	password := explicit.Password

	if username != "" && password != "" {
		return HarborCredentials{Username: username, Password: password}, nil
	}

	kcUser, kcPass, ok, err := keychainCredentials(tgt)
	if err != nil {
		return HarborCredentials{}, err
	}

	if ok {
		username = firstNonEmpty(username, kcUser)
		password = firstNonEmpty(password, kcPass)
	}

	if username == "" || password == "" {
		return HarborCredentials{}, fmt.Errorf("no Harbor credentials found for %s", tgt.Hostname())
	}

	return HarborCredentials{Username: username, Password: password}, nil
}

// firstNonEmpty returns the first non-empty value.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}

// redact prevents secrets from appearing in formatted credentials.
func redact(secret string) string {
	if secret == "" {
		return `""`
	}

	return "***"
}
