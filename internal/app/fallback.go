// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"context"
	"errors"
	"unicode/utf8"

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
	source := doc.Content
	if err := renderDocument(&doc, format); err != nil {
		return err
	}

	if providerType == provider.DockerHub {
		if format == document.FormatHTML {
			original := len(doc.Content)
			var err error
			source, err = limitRenderedDocument(
				source, &doc, dockerHubBodyLimit, headingLevel, format, countRunes,
			)
			if err != nil {
				return err
			}
			if len(doc.Content) < original {
				reporter.Warnf(
					"document body exceeds the provider limit, cutting from %d to %d bytes",
					original, len(doc.Content),
				)
			}
		} else {
			limitDocumentRunes(&doc, dockerHubBodyLimit, reporter)
		}
	}

	if bodyLimit > 0 {
		if format == document.FormatHTML {
			original := len(doc.Content)
			var err error
			source, err = limitRenderedDocument(
				source, &doc, bodyLimit, headingLevel, format, countBytes,
			)
			if err != nil {
				return err
			}
			if len(doc.Content) < original {
				reporter.Warnf(
					"document body exceeds --doc-body-limit, cutting from %d to %d bytes",
					original, len(doc.Content),
				)
			}
		} else {
			cutDocument(&doc, bodyLimit, headingLevel, reporter)
		}
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

		var nextSource, nextContent []byte
		if format == document.FormatHTML {
			var renderErr error
			nextSource, nextContent, renderErr = cutAndRender(
				source, doc.Content, nextLimit, headingLevel, format, countBytes,
			)
			if renderErr != nil {
				return renderErr
			}
		} else {
			nextContent = document.Cut(doc.Content, nextLimit, headingLevel)
		}
		if len(nextContent) >= len(doc.Content) {
			return err
		}

		reporter.Warnf(
			"payload too large, cutting document from %d to %d bytes and retrying",
			len(doc.Content), len(nextContent),
		)
		doc.Content = nextContent
		if format == document.FormatHTML {
			source = nextSource
		}
	}
}

// limitRenderedDocument cuts the Markdown source
// and renders it again until the rendered result fits the requested limit.
func limitRenderedDocument(
	source []byte,
	doc *provider.Document,
	limit int,
	headingLevel int,
	format document.Format,
	measure func([]byte) int,
) ([]byte, error) {
	nextSource, nextContent, err := cutAndRender(
		source, doc.Content, limit, headingLevel, format, measure,
	)
	if err != nil {
		return nil, err
	}

	doc.Content = nextContent

	return nextSource, nil
}

// cutAndRender reduces source until its freshly rendered output fits limit.
func cutAndRender(
	source []byte,
	rendered []byte,
	limit int,
	headingLevel int,
	format document.Format,
	measure func([]byte) int,
) ([]byte, []byte, error) {
	for measure(rendered) > limit {
		nextLimit := len(source) * limit / measure(rendered)
		if nextLimit >= len(source) {
			nextLimit = len(source) - 1
		}
		if nextLimit <= 0 {
			source = nil
		} else {
			source = document.Cut(source, nextLimit, headingLevel)
		}

		candidate := provider.Document{Content: source}
		if err := renderDocument(&candidate, format); err != nil {
			return nil, nil, err
		}
		rendered = candidate.Content
	}

	return source, rendered, nil
}

// countBytes returns the encoded payload size.
func countBytes(content []byte) int {
	return len(content)
}

// countRunes returns the provider-visible character count.
func countRunes(content []byte) int {
	return utf8.RuneCount(content)
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
