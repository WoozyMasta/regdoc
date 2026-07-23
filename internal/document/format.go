// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// Format selects the representation sent to a registry or local output.
type Format string

// Supported output formats.
const (
	FormatMarkdown Format = "md"   // FormatMarkdown preserves the normalized Markdown document.
	FormatHTML     Format = "html" // FormatHTML renders the normalized Markdown document as HTML.
)

// Render converts normalized Markdown to format.
// Markdown is returned unchanged;
// HTML enables tables because some registry UIs commonly support HTML
// even when their Markdown dialect does not support GFM tables.
func Render(content []byte, format Format) ([]byte, error) {
	switch format {
	case "", FormatMarkdown:
		return content, nil
	case FormatHTML:
		var buf bytes.Buffer

		// Markdown uses data&colon; to accommodate registry sanitizers; HTML needs data:.
		content = []byte(strings.ReplaceAll(string(content), "data&colon;", "data:"))

		markdown := goldmark.New(goldmark.WithExtensions(extension.Table))
		if err := markdown.Convert(content, &buf); err != nil {
			return nil, fmt.Errorf("render HTML: %w", err)
		}

		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}
