// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"errors"
	"strings"
	"testing"
)

func TestResolvePasswordFromStdin(t *testing.T) {
	cfg := Config{AuthenticationOptions: AuthenticationOptions{PasswordStdin: true}}

	if err := cfg.ResolveSecrets(strings.NewReader("secret-pass\n")); err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}

	if cfg.Password != "secret-pass" || cfg.Token != "" {
		t.Fatalf("got Password=%q Token=%q", cfg.Password, cfg.Token)
	}
}

func TestResolveTokenFromStdin(t *testing.T) {
	cfg := Config{AuthenticationOptions: AuthenticationOptions{TokenStdin: true}}

	if err := cfg.ResolveSecrets(strings.NewReader("secret-token\n")); err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}

	if cfg.Password != "" || cfg.Token != "secret-token" {
		t.Fatalf("got Password=%q Token=%q", cfg.Password, cfg.Token)
	}
}

func TestResolveSecretsNoStdinFlagsLeavesValuesUntouched(t *testing.T) {
	cfg := Config{AuthenticationOptions: AuthenticationOptions{Password: "cli-pass", Token: "cli-token"}}

	if err := cfg.ResolveSecrets(strings.NewReader("")); err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}

	if cfg.Password != "cli-pass" || cfg.Token != "cli-token" {
		t.Fatalf("got Password=%q Token=%q", cfg.Password, cfg.Token)
	}
}

// clearBaseURLEnv isolates automatic source URL detection in each test.
func clearBaseURLEnv(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"CI_PROJECT_URL",
		"CI_DEFAULT_BRANCH",
		"GITHUB_SERVER_URL",
		"GITHUB_REPOSITORY",
		"GITHUB_SHA",
	} {
		t.Setenv(name, "")
	}
}

func TestResolveBaseURLExplicitWins(t *testing.T) {
	clearBaseURLEnv(t)
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example/group/project")
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	got := resolveBaseURL("https://explicit.example/")
	if got != "https://explicit.example/" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveBaseURLFromCI(t *testing.T) {
	clearBaseURLEnv(t)
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example/group/project")
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	got := resolveBaseURL("")
	want := "https://gitlab.example/group/project/-/raw/main/"

	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveBaseURLFromGitHubActions(t *testing.T) {
	clearBaseURLEnv(t)
	t.Setenv("GITHUB_SERVER_URL", "https://github.example")
	t.Setenv("GITHUB_REPOSITORY", "group/project")
	t.Setenv("GITHUB_SHA", "0123456789abcdef")

	got := resolveBaseURL("")
	want := "https://github.example/group/project/raw/0123456789abcdef/"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestResolveBaseURLFromBitbucketPipelines(t *testing.T) {
	clearBaseURLEnv(t)
	t.Setenv("BITBUCKET_GIT_HTTP_ORIGIN", "https://bitbucket.example/workspace/project.git")
	t.Setenv("BITBUCKET_COMMIT", "0123456789abcdef")

	got := resolveBaseURL("")
	want := "https://bitbucket.example/workspace/project/raw/0123456789abcdef/"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestResolveBaseURLNoCIVars(t *testing.T) {
	clearBaseURLEnv(t)
	t.Setenv("CI_PROJECT_URL", "")
	t.Setenv("CI_DEFAULT_BRANCH", "")

	if got := resolveBaseURL(""); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestConfigErrorUnwrap(t *testing.T) {
	err := configErrorf("wrap: %w", errors.New("boom"))

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatal("expected configErrorf to produce a *ConfigError")
	}

	if ce.Error() != "wrap: boom" {
		t.Fatalf("Error() = %q", ce.Error())
	}
}
