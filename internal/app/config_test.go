// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/woozymasta/flags"
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

// newTestParser builds a parser matching cmd/regdoc/main.go's env provisioning setup,
// without the interactive-only options (help, version, shell completion) that would complicate these tests.
func newTestParser(cfg *Config) *flags.Parser {
	parser := flags.NewParser(cfg, flags.EnvProvisioning)
	parser.SetEnvPrefix("REGDOC")

	return parser
}

// asFlagsError unwraps err into a *flags.Error, failing the test if it isn't one.
func asFlagsError(t *testing.T, err error) *flags.Error {
	t.Helper()

	var flagsErr *flags.Error
	if !errors.As(err, &flagsErr) {
		t.Fatalf("expected *flags.Error, got %T: %v", err, err)
	}

	return flagsErr
}

func TestLinkAndImageBaseURLOnlyOneSetIsError(t *testing.T) {
	var cfg Config

	_, err := newTestParser(&cfg).ParseArgs([]string{"--link-base-url=https://git.example/project/-/blob/main/", "image"})
	if err == nil {
		t.Fatal("expected error when only --link-base-url is set")
	}

	if got := asFlagsError(t, err); got.Type != flags.ErrOptionRequirement {
		t.Fatalf("Type = %v, want ErrOptionRequirement", got.Type)
	}
}

func TestLinkAndImageBaseURLOnlyImageEnvSetIsError(t *testing.T) {
	t.Setenv("REGDOC_IMAGE_BASE_URL", "https://git.example/project/-/raw/main/")

	var cfg Config

	_, err := newTestParser(&cfg).ParseArgs([]string{"image"})
	if err == nil {
		t.Fatal("expected error when only REGDOC_IMAGE_BASE_URL is set")
	}

	if got := asFlagsError(t, err); got.Type != flags.ErrOptionRequirement {
		t.Fatalf("Type = %v, want ErrOptionRequirement", got.Type)
	}
}

func TestLinkAndImageBaseURLBothSetIsNotAnError(t *testing.T) {
	var cfg Config

	_, err := newTestParser(&cfg).ParseArgs([]string{
		"--link-base-url=https://git.example/project/-/blob/main/",
		"--image-base-url=https://git.example/project/-/raw/main/",
		"image",
	})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}

	if cfg.LinkBaseURL == "" || cfg.ImageBaseURL == "" {
		t.Fatalf("got LinkBaseURL=%q ImageBaseURL=%q", cfg.LinkBaseURL, cfg.ImageBaseURL)
	}
}

func TestLinkAndImageBaseURLBothAbsentIsNotAnError(t *testing.T) {
	var cfg Config

	if _, err := newTestParser(&cfg).ParseArgs([]string{"image"}); err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}

	if cfg.LinkBaseURL != "" || cfg.ImageBaseURL != "" {
		t.Fatalf("got LinkBaseURL=%q ImageBaseURL=%q, want both empty", cfg.LinkBaseURL, cfg.ImageBaseURL)
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
