// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"bytes"
	"fmt"

	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

// FinalizeAnchors makes fragment links absolute for providers with a known repository UI URL.
// Other providers keep the original content.
func FinalizeAnchors(content []byte, providerType provider.Type, tgt target.Target) ([]byte, error) {
	base := anchorBaseURL(providerType, tgt)
	if base == "" {
		return content, nil
	}

	renderer := markdown.NewRenderer()
	md := goldmark.New(goldmark.WithRenderer(renderer))

	reader := text.NewReader(content)
	doc := md.Parser().Parse(reader)

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		var dest *[]byte

		switch node := n.(type) { //nolint:exhaustive // only Link/Image carry a destination.
		case *ast.Link:
			dest = &node.Destination

		case *ast.Image:
			dest = &node.Destination

		default:
			return ast.WalkContinue, nil
		}

		if bytes.HasPrefix(*dest, []byte("#")) {
			*dest = []byte(base + string(*dest))
		}

		return ast.WalkContinue, nil
	})

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, content, doc); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}

	return buf.Bytes(), nil
}

// anchorBaseURL returns a provider UI URL when it can be derived locally.
func anchorBaseURL(providerType provider.Type, tgt target.Target) string {
	switch providerType {
	case provider.Quay:
		return "https://" + tgt.Registry + "/repository/" + tgt.Repository

	case provider.DockerHub:
		// library/<name> official images are left as-is here;
		// the fragment survives Docker Hub's redirect to hub.docker.com/_/<name>.
		return "https://hub.docker.com/r/" + tgt.Repository

	case provider.Harbor, provider.Unknown:
		return ""

	default:
		return ""
	}
}
