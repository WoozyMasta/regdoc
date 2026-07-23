// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"context"
	"errors"

	"github.com/woozymasta/regdoc/internal/document"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

// FallbackNone disables payload cutting after a size failure.
const FallbackNone = "none"

// FallbackCut retries a size failure with a smaller document body.
const FallbackCut = "cut"

// dockerHubBodyLimit is the documented maximum number of characters
// in a Docker Hub full_description field.
const dockerHubBodyLimit = 25000

// publish renders and publishes a document,
// cutting its body only after a classified size failure.
// Network and API failures are never retried here.
func publish(
	ctx context.Context,
	pub provider.Publisher,
	tgt target.Target,
	header []byte,
	parts []document.ProcessedPart,
	shortDescription string,
	fallbackMode string,
	bodyLimit int,
	headingLevel int,
	cutRetries int,
	providerType provider.Type,
	format document.Format,
	reporter Reporter,
) error {
	doc := document.Merge(header, parts)
	doc.ShortDescription = shortDescription
	if err := renderDocument(&doc, format); err != nil {
		return err
	}

	if providerType == provider.DockerHub {
		limitDocumentRunes(&doc, dockerHubBodyLimit, reporter)
	}

	if bodyLimit > 0 {
		cutDocument(&doc, bodyLimit, headingLevel, reporter)
	}

	for attempt := 0; ; attempt++ {
		reporter.Debugf("publishing sources=%v size=%d bytes", doc.Sources, len(doc.Content))

		err := pub.Publish(ctx, tgt, doc)
		if err == nil {
			return nil
		}

		if fallbackMode != FallbackCut || !errors.Is(err, provider.ErrPayloadTooLarge) {
			return err
		}

		if attempt >= cutRetries {
			return err
		}

		nextLimit := len(doc.Content) * 3 / 4
		if nextLimit >= len(doc.Content) {
			nextLimit = len(doc.Content) - 1
		}
		if nextLimit <= 0 {
			return err
		}

		nextContent := document.Cut(doc.Content, nextLimit, headingLevel)
		if len(nextContent) >= len(doc.Content) {
			return err
		}

		reporter.Warnf(
			"payload too large, cutting document from %d to %d bytes and retrying",
			len(doc.Content), len(nextContent),
		)
		doc.Content = nextContent
	}
}

// limitDocumentRunes applies a provider field-length limit before publishing.
func limitDocumentRunes(doc *provider.Document, limit int, reporter Reporter) {
	original := len(doc.Content)
	doc.Content = document.LimitRunes(doc.Content, limit)
	if len(doc.Content) < original {
		reporter.Warnf(
			"document body exceeds the provider limit, cutting from %d to %d bytes",
			original, len(doc.Content),
		)
	}
}

// cutDocument applies a user-configured byte limit before the first request.
func cutDocument(doc *provider.Document, limit, headingLevel int, reporter Reporter) {
	original := len(doc.Content)
	doc.Content = document.Cut(doc.Content, limit, headingLevel)
	if len(doc.Content) < original {
		reporter.Warnf(
			"document body exceeds --doc-body-limit, cutting from %d to %d bytes",
			original, len(doc.Content),
		)
	}
}
