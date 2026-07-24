// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/woozymasta/regdoc/internal/auth"
	"github.com/woozymasta/regdoc/internal/document"
	"github.com/woozymasta/regdoc/internal/httpx"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/source"
	"github.com/woozymasta/regdoc/internal/target"
)

// Run builds repository documentation and publishes it or writes local output.
func Run(ctx context.Context, cfg Config, stdout, stderr io.Writer) error {
	reporter := NewReporter(stderr, cfg.Debug, cfg.Quiet)

	// Validate the target before touching the filesystem or network.
	tgt, err := target.Parse(cfg.Positional.Image)
	if err != nil {
		return configErrorf("parse image reference %q: %w", cfg.Positional.Image, err)
	}

	// Source discovery is a no-op when no default documents exist.
	sources, err := document.Discover(cfg.Root, cfg.Positional.Markdown)
	if err != nil {
		return configErrorf("discover documents: %w", err)
	}

	if len(sources) == 0 {
		reporter.Debugf("no documents found under %q, nothing to publish", cfg.Root)

		return nil
	}

	// TLS policy applies to both probing and provider API requests.
	scheme := "https"
	if cfg.PlainHTTP {
		scheme = "http"
	}

	httpClient := httpx.NewClient(httpx.Options{
		Timeout:       cfg.Timeout,
		TLSSkipVerify: cfg.TLSSkipVerify,
		UserAgent:     cfg.UserAgent,
	})

	// Provider type controls anchor rendering and publisher selection.
	providerType, err := resolveProviderType(ctx, cfg, tgt, scheme, httpClient, reporter)
	if err != nil {
		return err
	}

	// A local destination never resolves credentials or publishes;
	// only the publish path needs a Publisher, so credentials
	// and the tag-order gate are resolved here, before any markdown rendering,
	// so a stale tag never pays for work whose result would be discarded.
	var pub provider.Publisher

	if cfg.Output == "" {
		explicit := auth.Explicit{Username: cfg.Username, Password: cfg.Password, Token: cfg.Token}

		pub, err = newPublisher(httpClient, providerType, scheme, explicit, tgt)
		if err != nil {
			if cfg.Optional {
				reporter.Infof("optional: %v", err)
				return nil
			}

			return err
		}

		if !cfg.SkipTagCheck {
			if lister, ok := pub.(provider.TagLister); ok {
				skip, gerr := tagOrderGate(ctx, cfg, lister, tgt, reporter)
				if gerr != nil {
					return gerr
				}

				if skip {
					return nil
				}
			}
		}
	}

	// Source discovery is resolved once from CI environment variables
	// and reused for both header metadata and link/image base URLs.
	resolved := source.Resolve(os.Getenv)

	// An explicit --release-version wins over IMAGE's tag; neither is required.
	// The link is only built when both the version and a tag-page base URL are known.
	releaseVersion := cfg.ReleaseVersion
	if releaseVersion == "" {
		releaseVersion, _ = target.ExplicitTag(cfg.Positional.Image)
	}

	var releaseURL string
	if releaseVersion != "" && resolved.ReleaseBaseURL != "" {
		releaseURL = resolved.ReleaseBaseURL + url.PathEscape(releaseVersion)
	}

	// Header metadata is resolved once and reused by fallback attempts.
	header, err := document.BuildHeader(document.HeaderConfig{
		Root:            cfg.Root,
		Title:           cfg.Title,
		SourceName:      cfg.SourceName,
		SourceURL:       cfg.SourceURL,
		Author:          cfg.Author,
		Copyright:       cfg.Copyright,
		License:         cfg.License,
		Release:         releaseVersion,
		ReleaseURL:      releaseURL,
		DiscoveredName:  resolved.Name,
		DiscoveredTitle: resolved.Title,
		DiscoveredURL:   resolved.ProjectURL,
	})
	if err != nil {
		return configErrorf("build header: %w", err)
	}

	// The `and` relation on --link-base-url/--image-base-url guarantees both are set
	// or both are empty by the time Run executes, so checking one is enough.
	linkBaseURL, imageBaseURL := cfg.LinkBaseURL, cfg.ImageBaseURL
	if linkBaseURL == "" {
		linkBaseURL, imageBaseURL = resolved.LinkBaseURL, resolved.ImageBaseURL
	}

	// Render sources once; retries only re-merge the rendered parts.
	parts, err := document.Process(sources, document.BuildConfig{
		Root:          cfg.Root,
		LinkBaseURL:   linkBaseURL,
		ImageBaseURL:  imageBaseURL,
		EmbedImages:   cfg.EmbedImages,
		StripComments: !cfg.KeepComments,
	})
	if err != nil {
		return fmt.Errorf("process documents: %w", err)
	}

	for i, p := range parts {
		content, aerr := document.FinalizeAnchors([]byte(p.Content), providerType, tgt)
		if aerr != nil {
			return fmt.Errorf("finalize anchors in %q: %w", p.Path, aerr)
		}

		parts[i].Content = string(content)
	}

	// A local destination never resolves credentials or publishes.
	if cfg.Output != "" {
		doc := document.Merge(header, parts)
		if err := renderDocument(&doc, cfg.Format); err != nil {
			return err
		}

		return writeResult(cfg, stdout, doc.Content, sourcePaths(sources))
	}

	// Payload cutting is enforced only for size failures from the provider.
	if err := publish(
		ctx, pub, tgt, header, parts,
		cfg.ShortDescription,
		cfg.Fallback,
		cfg.DocBodyLimit,
		cfg.CutHeadingLevel,
		cfg.CutRetries,
		providerType,
		cfg.Format,
		reporter,
	); err != nil {
		if cfg.Optional && errors.Is(err, provider.ErrNotFound) {
			reporter.Infof("optional: repository not found on %s: %v", providerType, err)
			return nil
		}

		return err
	}

	return nil
}

// renderDocument converts a merged document immediately before output or
// publishing, after link rewriting and provider-specific anchor finalization.
func renderDocument(doc *provider.Document, format document.Format) error {
	content, err := document.Render(doc.Content, format)
	if err != nil {
		return fmt.Errorf("render document as %s: %w", format, err)
	}

	doc.Content = content

	return nil
}

// resolveProviderType avoids probing when configuration or a known hostname is sufficient.
// Local output never performs provider-detection requests.
func resolveProviderType(
	ctx context.Context,
	cfg Config,
	tgt target.Target,
	scheme string,
	httpClient *http.Client,
	reporter Reporter,
) (provider.Type, error) {
	if cfg.Provider != "auto" {
		return provider.Type(cfg.Provider), nil
	}

	if t, ok := provider.KnownHost(tgt.Hostname()); ok {
		return t, nil
	}

	// A local destination never resolves credentials or publishes.
	if cfg.Output != "" {
		reporter.Debugf("local output: skipping provider autodetection for %s", tgt.Hostname())

		return provider.Unknown, nil
	}

	detector := &provider.Detector{
		Client:    httpClient,
		Scheme:    scheme,
		UserAgent: cfg.UserAgent,
		Timeout:   min(provider.DetectTimeout, cfg.Timeout),
	}

	t, results, err := detector.Detect(ctx, tgt)
	if cfg.Debug {
		for _, r := range results {
			reporter.Debugf(
				"probe %s: matched=%v status=%d err=%v",
				r.Provider, r.Matched, r.StatusCode, r.Err,
			)
		}
	}

	if err != nil {
		return provider.Unknown, err
	}

	return t, nil
}

// sourcePaths returns source paths in merge order.
func sourcePaths(sources []document.Source) []string {
	paths := make([]string, len(sources))
	for i, s := range sources {
		paths[i] = s.Path
	}

	return paths
}

// writeResult writes local output to stdout or a file destination.
func writeResult(cfg Config, stdout io.Writer, content []byte, inputPaths []string) error {
	if cfg.Output == "-" {
		_, err := stdout.Write(content)
		return err
	}

	return WriteOutput(cfg.Output, content, inputPaths)
}
