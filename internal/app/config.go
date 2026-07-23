// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/woozymasta/regdoc/internal/document"
)

// Config is the CLI schema and the resolved application configuration.
// Long flags map to REGDOC_* environment variables through EnvProvisioning.
type Config struct { // betteralign:ignore
	Positional Positional `positional-args:"yes"`

	OutputOptions         `group:"Output Options" description:"Repository description format, size fallback and local output."`
	DocumentOptions       `group:"Document Options" description:"Input documents, link rewriting and generated header metadata."`
	TargetOptions         `group:"Target Options" description:"Registry destination and transport settings."`
	AuthenticationOptions `group:"Authentication Options" description:"Credentials used only when publishing to a registry."`
	RuntimeOptions        `group:"Runtime Options" description:"Request timeout, no-op behavior and diagnostics."`
}

// TargetOptions selects the destination registry and source repository.
type TargetOptions struct {
	Provider      string `long:"provider"        description:"Provider: auto, dockerhub, quay or harbor" short:"p" default:"auto" choices:"auto;dockerhub;quay;harbor"`
	Root          string `long:"root"            description:"Root directory for document discovery and relative link resolution" short:"r" default:"." completion:"dir" validate-readable:"yes" validate-existing-dir:"yes"`
	PlainHTTP     bool   `long:"plain-http"      description:"Use plain HTTP for the target registry" xor:"transport"`
	TLSSkipVerify bool   `long:"tls-skip-verify" description:"Disable TLS certificate verification for the target registry" xor:"transport"`
}

// DocumentOptions controls document discovery, rewriting and generated metadata.
type DocumentOptions struct {
	Title        string `long:"title"          description:"Project title used in the generated header"`
	SourceName   string `long:"source-name"    description:"Project name shown in the generated header"`
	SourceURL    string `long:"source-url"     description:"Project URL shown in the generated header"`
	License      string `long:"license"        description:"License file to identify in the header (default: auto-discover LICENSE in --root)" completion:"file" validate-readable:"yes" validate-existing-file:"yes"`
	Author       string `long:"author"         description:"Author shown in the generated header"`
	Copyright    string `long:"copyright"      description:"Copyright notice shown in the generated header"`
	LinkBaseURL  string `long:"link-base-url"  description:"Base URL prepended to relative Markdown link destinations; overrides CI URL discovery (requires --image-base-url)" and:"source-base-url"`
	ImageBaseURL string `long:"image-base-url" description:"Base URL prepended to relative Markdown image destinations; overrides CI URL discovery (requires --link-base-url)" and:"source-base-url"`
	KeepComments bool   `long:"keep-comments"  description:"Keep HTML comments instead of stripping them from the published Markdown"`
}

// OutputOptions controls the generated repository description.
type OutputOptions struct {
	ShortDescription string          `long:"short-description" description:"Short description, where supported by the provider"`
	Fallback         string          `long:"fallback"          description:"Behavior when the payload is too large"                                       default:"cut" choices:"none;cut"`
	Output           string          `long:"output"            description:"Write generated content without publishing; use - for stdout" short:"o" completion:"file"`
	Format           document.Format `long:"format"            description:"Output format: md or html"                                                    default:"md" choices:"md;html"`
	DocBodyLimit     int             `long:"doc-body-limit"    description:"Maximum rendered document body size in bytes; zero uses the provider default" default:"0" validate-min:"0"`
	CutHeadingLevel  int             `long:"cut-heading-level" description:"Prefer a heading from level 1 through this level as the cut boundary"         default:"2" validate-min:"1" validate-max:"6"`
	CutRetries       int             `long:"cut-retries"       description:"Maximum additional publish attempts after a payload-too-large response"       default:"5" validate-min:"0" validate-max:"20"`
	EmbedImages      bool            `long:"embed-images"      description:"Embed local images as base64 data URIs"`
}

// AuthenticationOptions provides explicit registry credentials.
type AuthenticationOptions struct {
	Username      string `long:"username"       description:"Registry username"`
	Password      string `long:"password"       description:"Registry password (prefer --password-stdin or REGDOC_PASSWORD: CLI arguments are visible to other processes)" xor:"creds;pswd" secret:"yes"`
	Token         string `long:"token"          description:"API token (prefer --token-stdin or REGDOC_TOKEN: CLI arguments are visible to other processes)"               xor:"creds;token" secret:"yes"`
	PasswordStdin bool   `long:"password-stdin" description:"Read the password from stdin"                                                                                 xor:"creds;pswd"`
	TokenStdin    bool   `long:"token-stdin"    description:"Read the token from stdin"                                                                                    xor:"creds;token"`
}

// RuntimeOptions controls request execution and diagnostics.
type RuntimeOptions struct {
	UserAgent string        `no-flag:"yes"`
	Timeout   time.Duration `long:"timeout"  description:"Per-request HTTP timeout" default:"30s"`
	Optional  bool          `long:"optional" description:"Treat missing documents, credentials or repository as a successful no-op"`
	Debug     bool          `long:"debug"    description:"Print technical diagnostics to stderr" xor:"verbosity"`
	Quiet     bool          `long:"quiet"    description:"Suppress informational output (errors are still printed)" xor:"verbosity" short:"q"`
}

// Positional contains positional command arguments.
type Positional struct {
	Image    string   `positional-arg-name:"IMAGE"    description:"Container repository reference" required:"yes"`
	Markdown []string `positional-arg-name:"MARKDOWN" description:"Markdown files or glob patterns to publish, in order" completion:"file"`
}

// ConfigError maps a configuration failure to exit code 2.
type ConfigError struct{ Err error }

// Error implements error.
func (e *ConfigError) Error() string { return e.Err.Error() }

// Unwrap implements error unwrapping.
func (e *ConfigError) Unwrap() error { return e.Err }

// ResolveSecrets reads one selected stdin credential.
func (c *Config) ResolveSecrets(stdin io.Reader) error {
	buf := bufio.NewReader(stdin)

	if c.PasswordStdin {
		v, err := readStdinSecret(buf)
		if err != nil {
			return configErrorf("read --password-stdin: %w", err)
		}

		c.Password = v

		return nil
	}

	if c.TokenStdin {
		v, err := readStdinSecret(buf)
		if err != nil {
			return configErrorf("read --token-stdin: %w", err)
		}

		c.Token = v
	}

	return nil
}

// readStdinSecret reads one newline-delimited secret.
func readStdinSecret(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}

	return strings.TrimRight(line, "\r\n"), nil
}

// configErrorf constructs an error with configuration exit semantics.
func configErrorf(format string, args ...any) error {
	return &ConfigError{Err: fmt.Errorf(format, args...)}
}
