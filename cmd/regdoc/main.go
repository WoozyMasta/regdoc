// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package main implements the regdoc command.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/woozymasta/flags"

	"github.com/woozymasta/regdoc/internal/app"
	"github.com/woozymasta/regdoc/internal/version"
)

// Process exit codes.
const (
	exitSuccess = iota // exitSuccess reports successful command completion.
	exitRuntime        // exitRuntime reports an operational failure.
	exitConfig         // exitConfig reports invalid CLI configuration.
)

// main exits with the result of run.
func main() {
	os.Exit(run())
}

// run parses CLI input and executes the application.
func run() int {
	var cfg app.Config

	parserOptions := flags.Options(flags.Default |
		flags.DetectShellFlagStyle |
		flags.DetectShellEnvStyle |
		flags.PrintHelpOnInputErrors |
		flags.HelpCommand |
		flags.VersionCommand |
		flags.CompletionCommand |
		flags.DocsCommand |
		flags.VersionFlag |
		flags.StrictPositionalArgs |
		flags.ShowRepeatableInHelp |
		flags.EnvProvisioning)

	parser := flags.NewParser(&cfg, parserOptions)
	parser.SetShortDescription("Publish repository documentation to container registry descriptions.")
	parser.SetLongDescription("regdoc collects repository Markdown and publishes it as the description of a Docker Hub, Quay or Harbor container registry.")
	parser.SetVersion(version.Version)
	parser.SetVersionCommit(version.Commit)
	parser.SetVersionURL(version.URL)
	parser.SetEnvPrefix("REGDOC")
	parser.SetSubcommandsOptional(true)

	if t, err := time.Parse(time.RFC3339, version.BuildTime); err == nil {
		parser.SetVersionTime(t)
	}

	if _, err := parser.Parse(); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && (flagsErr.Type == flags.ErrHelp || flagsErr.Type == flags.ErrVersion) {
			return exitSuccess
		}

		return exitConfig
	}

	// Built-in commands execute during parsing and must not fall through to publishing.
	if parser.Active != nil {
		return exitSuccess
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg.UserAgent = version.UserAgent()

	if err := cfg.ResolveSecrets(os.Stdin); err != nil {
		return reportError(err)
	}

	return reportError(app.Run(ctx, cfg, os.Stdout, os.Stderr))
}

// reportError writes one diagnostic and maps it to a process exit code.
func reportError(err error) int {
	if err == nil {
		return exitSuccess
	}

	fmt.Fprintln(os.Stderr, "error:", err) //nolint:errcheck // best-effort diagnostics on the way out.

	var configErr *app.ConfigError
	if errors.As(err, &configErr) {
		return exitConfig
	}

	return exitRuntime
}
