// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package auth

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woozymasta/regdoc/internal/target"
)

// withDockerConfig points DOCKER_CONFIG at a fresh temp dir containing the given auths,
// restoring the previous env value afterwards.
func withDockerConfig(t *testing.T, auths map[string]string) {
	t.Helper()

	dir := t.TempDir()

	type authEntry struct {
		Auth string `json:"auth"`
	}

	cfg := struct {
		Auths map[string]authEntry `json:"auths"`
	}{Auths: map[string]authEntry{}}

	for host, userpass := range auths {
		cfg.Auths[host] = authEntry{Auth: base64.StdEncoding.EncodeToString([]byte(userpass))}
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal docker config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600); err != nil {
		t.Fatalf("write docker config: %v", err)
	}

	t.Setenv("DOCKER_CONFIG", dir)
}

// clearAuthEnv removes explicit credential variables for a test.
func clearAuthEnv(t *testing.T) {
	t.Helper()

	for _, k := range []string{
		"REGDOC_USERNAME", "REGDOC_PASSWORD", "REGDOC_TOKEN",
	} {
		t.Setenv(k, "")
	}
}

func TestResolveDockerHubExplicitCLI(t *testing.T) {
	clearAuthEnv(t)
	withDockerConfig(t, nil)

	got, err := ResolveDockerHub(
		Explicit{Username: "cli-user", Token: "cli-token"},
		target.Target{Registry: "index.docker.io"},
	)
	if err != nil {
		t.Fatalf("ResolveDockerHub: %v", err)
	}

	if got.Username != "cli-user" || got.Token != "cli-token" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveDockerHubKeychainInlineAuth(t *testing.T) {
	clearAuthEnv(t)
	withDockerConfig(t, map[string]string{
		"https://index.docker.io/v1/": "kc-user:kc-pass",
	})

	got, err := ResolveDockerHub(Explicit{}, target.Target{Registry: "index.docker.io"})
	if err != nil {
		t.Fatalf("ResolveDockerHub: %v", err)
	}

	if got.Username != "kc-user" || got.Token != "kc-pass" {
		t.Fatalf("expected keychain credentials, got %+v", got)
	}
}

func TestResolveDockerHubKeychainDockerIOAlias(t *testing.T) {
	clearAuthEnv(t)
	withDockerConfig(t, map[string]string{
		"https://index.docker.io/v1/": "alias-user:alias-pass",
	})

	got, err := ResolveDockerHub(Explicit{}, target.Target{Registry: "docker.io"})
	if err != nil {
		t.Fatalf("ResolveDockerHub: %v", err)
	}

	if got.Username != "alias-user" || got.Token != "alias-pass" {
		t.Fatalf("expected docker.io alias credentials, got %+v", got)
	}
}

func TestResolveDockerHubNoCredentials(t *testing.T) {
	clearAuthEnv(t)
	withDockerConfig(t, nil)

	if _, err := ResolveDockerHub(Explicit{}, target.Target{Registry: "index.docker.io"}); err == nil {
		t.Fatal("expected error when no Docker Hub credentials are available")
	}
}

func TestResolveQuayTokenIsolation(t *testing.T) {
	clearAuthEnv(t)
	// A registry password/keychain entry must never be treated as a Quay OAuth token.
	withDockerConfig(t, map[string]string{"https://quay.io/v1/": "user:not-a-token"})

	if _, err := ResolveQuay(Explicit{}); err == nil {
		t.Fatal("expected error: Quay must not fall back to Docker keychain credentials")
	}

	got, err := ResolveQuay(Explicit{Token: "explicit-token"})
	if err != nil {
		t.Fatalf("ResolveQuay: %v", err)
	}

	if got.Token != "explicit-token" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveHarborKeychain(t *testing.T) {
	clearAuthEnv(t)
	withDockerConfig(t, map[string]string{"harbor.example.com": "hu:hp"})

	got, err := ResolveHarbor(Explicit{}, target.Target{Registry: "harbor.example.com"})
	if err != nil {
		t.Fatalf("ResolveHarbor: %v", err)
	}

	if got.Username != "hu" || got.Password != "hp" {
		t.Fatalf("got %+v", got)
	}

	// Explicit carries whatever the flags parser already resolved
	// from --username/--password or REGDOC_USERNAME/REGDOC_PASSWORD;
	// it must win over the keychain.
	got, err = ResolveHarbor(
		Explicit{Username: "explicit-user", Password: "explicit-pass"},
		target.Target{Registry: "harbor.example.com"},
	)
	if err != nil {
		t.Fatalf("ResolveHarbor: %v", err)
	}

	if got.Username != "explicit-user" || got.Password != "explicit-pass" {
		t.Fatalf("expected explicit value to win over keychain, got %+v", got)
	}
}

func TestResolveHarborNoCredentials(t *testing.T) {
	clearAuthEnv(t)
	withDockerConfig(t, nil)

	if _, err := ResolveHarbor(Explicit{}, target.Target{Registry: "harbor.example.com"}); err == nil {
		t.Fatal("expected error when no Harbor credentials are available")
	}
}

func TestCredentialsStringRedaction(t *testing.T) {
	dh := DockerHubCredentials{Username: "u", Token: "super-secret-token"}
	if strings.Contains(dh.String(), "super-secret-token") {
		t.Fatalf("DockerHubCredentials.String() leaked the token: %s", dh)
	}

	q := QuayCredentials{Token: "super-secret-token"}
	if strings.Contains(q.String(), "super-secret-token") {
		t.Fatalf("QuayCredentials.String() leaked the token: %s", q)
	}

	h := HarborCredentials{Username: "u", Password: "super-secret-password"}
	if strings.Contains(h.String(), "super-secret-password") {
		t.Fatalf("HarborCredentials.String() leaked the password: %s", h)
	}
}
