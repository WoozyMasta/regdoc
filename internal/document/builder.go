// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"fmt"
	"strings"

	"github.com/woozymasta/regdoc/internal/provider"
)

// partSeparator separates non-empty merged document parts.
const partSeparator = "\n\n---\n\n"

// BuildConfig configures per-source Markdown processing shared across all sources of a single build.
type BuildConfig struct {
	Root          string // Root bounds relative-link resolution.
	LinkBaseURL   string // LinkBaseURL prefixes rewritten relative *ast.Link destinations.
	ImageBaseURL  string // ImageBaseURL prefixes rewritten relative *ast.Image destinations.
	EmbedImages   bool   // EmbedImages replaces local image paths with data URIs.
	StripComments bool   // StripComments removes standalone HTML comments.
}

// ProcessedPart is one source after the (expensive)
// parse/rewrite/render pipeline ready to be merged. It is never empty.
type ProcessedPart struct {
	Path    string // Path identifies the source document.
	Content string // Content is rendered Markdown without a trailing newline.
}

// Process renders sources once so fallback retries can merge without reparsing.
func Process(sources []Source, cfg BuildConfig) ([]ProcessedPart, error) {
	parts := make([]ProcessedPart, 0, len(sources))

	for _, src := range sources {
		rewritten, err := Rewrite(src.Content, RewriteConfig{
			Root:          cfg.Root,
			RelPath:       src.RelPath,
			LinkBaseURL:   cfg.LinkBaseURL,
			ImageBaseURL:  cfg.ImageBaseURL,
			EmbedImages:   cfg.EmbedImages,
			StripComments: cfg.StripComments,
		})
		if err != nil {
			return nil, fmt.Errorf("process %q: %w", src.Path, err)
		}

		trimmed := strings.TrimRight(string(rewritten), "\n")
		if trimmed == "" {
			continue
		}

		parts = append(parts, ProcessedPart{Path: src.Path, Content: trimmed})
	}

	return parts, nil
}

// Merge omits empty parts and returns either empty content or one trailing newline.
func Merge(header []byte, parts []ProcessedPart) provider.Document {
	all := make([]string, 0, len(parts)+1)

	if trimmed := strings.TrimRight(string(header), "\n"); trimmed != "" {
		all = append(all, trimmed)
	}

	sourcePaths := make([]string, 0, len(parts))

	for _, p := range parts {
		all = append(all, p.Content)
		sourcePaths = append(sourcePaths, p.Path)
	}

	content := strings.Join(all, partSeparator)
	if content != "" {
		content += "\n"
	}

	return provider.Document{Content: []byte(content), Sources: sourcePaths}
}

// Build processes sources and merges them with header in one call.
// It is equivalent to Process followed by Merge.
func Build(sources []Source, header []byte, cfg BuildConfig) (provider.Document, error) {
	parts, err := Process(sources, cfg)
	if err != nil {
		return provider.Document{}, err
	}

	return Merge(header, parts), nil
}
