// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var htmlCommentPattern = regexp.MustCompile(`(?s)^<!--.*-->$`)

var embeddedImagePattern = regexp.MustCompile(
	`!\[([^\]\r\n]*)\]\((data:image/[^;\s)]+;base64,[A-Za-z0-9+/=]+)\)`,
)

// RewriteConfig configures per-source Markdown processing:
// link/image destination rewriting and HTML comment stripping.
type RewriteConfig struct {
	Root          string // Root bounds relative-link resolution.
	RelPath       string // RelPath locates the source under Root.
	BaseURL       string // BaseURL prefixes rewritten relative destinations.
	EmbedImages   bool   // EmbedImages replaces local image paths with data URIs.
	StripComments bool   // StripComments removes standalone HTML comments.
}

// Rewrite parses content as CommonMark, optionally strips HTML comments,
// rewrites relative link/image destinations per cfg, and renders the result back to Markdown.
// Fragment-only ("#anchor") destinations are always left untouched here;
// they are finalized separately once the provider is known.
func Rewrite(content []byte, cfg RewriteConfig) ([]byte, error) {
	renderer := markdown.NewRenderer()
	md := goldmark.New(goldmark.WithRenderer(renderer))

	reader := text.NewReader(content)
	doc := md.Parser().Parse(reader)

	// Remove comments at the AST level so code blocks and inline code survive.
	if cfg.StripComments {
		if err := stripComments(doc, content); err != nil {
			return nil, err
		}
	}

	// Rewrite only parsed link and image destinations.
	if err := rewriteLinks(doc, cfg); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, content, doc); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}

	return moveEmbeddedImagesToReferences(buf.Bytes()), nil
}

// moveEmbeddedImagesToReferences avoids inline data URI filtering in registry
// Markdown renderers that accept data URIs only in reference definitions.
func moveEmbeddedImagesToReferences(content []byte) []byte {
	matches := embeddedImagePattern.FindAllSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content
	}

	labels := make(map[string]string, len(matches))
	definitions := make([]string, 0, len(matches))
	result := embeddedImagePattern.ReplaceAllFunc(content, func(match []byte) []byte {
		submatches := embeddedImagePattern.FindSubmatch(match)
		altText := submatches[1]
		dataURI := string(submatches[2])
		label, ok := labels[dataURI]
		if !ok {
			label = fmt.Sprintf("regdoc-image-%d", len(labels)+1)
			labels[dataURI] = label
			definitions = append(definitions, "["+label+"]:data&colon;"+strings.TrimPrefix(dataURI, "data:"))
		}

		return []byte("![" + string(altText) + "][" + label + "]")
	})

	result = bytes.TrimRight(result, "\n")
	result = append(result, '\n', '\n')
	result = append(result, strings.Join(definitions, "\n")...)
	result = append(result, '\n')

	return result
}

// stripComments removes standalone HTML comment nodes only.
func stripComments(doc ast.Node, source []byte) error {
	var toRemove []ast.Node

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n.Kind() { //nolint:exhaustive // only comment-bearing kinds matter here.
		case ast.KindHTMLBlock, ast.KindRawHTML:
			if htmlCommentPattern.Match(nodeText(n, source)) {
				toRemove = append(toRemove, n)
			}
		}

		return ast.WalkContinue, nil
	})

	if err != nil {
		return fmt.Errorf("strip html comments: %w", err)
	}

	for _, n := range toRemove {
		if n.Parent() != nil {
			n.Parent().RemoveChild(n.Parent(), n)
		}
	}

	return nil
}

// nodeText returns an HTML node literal without surrounding whitespace.
func nodeText(n ast.Node, source []byte) []byte {
	var buf bytes.Buffer

	switch node := n.(type) {
	case *ast.HTMLBlock:
		lines := node.Lines()
		for i := range lines.Len() {
			seg := lines.At(i)
			buf.Write(seg.Value(source))
		}

	case *ast.RawHTML:
		segs := node.Segments
		for i := range segs.Len() {
			seg := segs.At(i)
			buf.Write(seg.Value(source))
		}
	}

	return bytes.TrimSpace(buf.Bytes())
}

// rewriteLinks applies destination rewriting to links and images.
func rewriteLinks(doc ast.Node, cfg RewriteConfig) error {
	return ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		var dest *[]byte
		isImage := false

		switch node := n.(type) { //nolint:exhaustive // only Link/Image carry a destination.
		case *ast.Link:
			dest = &node.Destination
		case *ast.Image:
			dest = &node.Destination
			isImage = true
		default:
			return ast.WalkContinue, nil
		}

		if isImage && cfg.EmbedImages {
			embedded, ok, err := embedImage(string(*dest), cfg)
			if err != nil {
				return ast.WalkStop, err
			}
			if ok {
				*dest = []byte(embedded)

				return ast.WalkContinue, nil
			}
		}

		newDest, err := rewriteDestination(string(*dest), cfg)
		if err != nil {
			return ast.WalkStop, err
		}

		*dest = []byte(newDest)

		return ast.WalkContinue, nil
	})
}

// embedImage reads a relative image inside Root and returns a base64 data URI.
// External, fragment-only and already scheme-qualified destinations are left unchanged.
func embedImage(dest string, cfg RewriteConfig) (string, bool, error) {
	if dest == "" || strings.HasPrefix(dest, "#") || strings.HasPrefix(dest, "//") {
		return "", false, nil
	}

	u, err := url.Parse(dest)
	isLocalPath := err == nil && u.Scheme == "" && u.Path != ""
	if !isLocalPath {
		return "", false, nil
	}

	imagePath, err := resolveLocalPath(cfg.Root, cfg.RelPath, u.Path)
	if err != nil {
		return "", false, fmt.Errorf("image %q in %q: %w", dest, cfg.RelPath, err)
	}

	content, err := os.ReadFile(imagePath) //nolint:gosec // path is bounded to --root by resolveLocalPath.
	if err != nil {
		return "", false, fmt.Errorf("read image %q: %w", dest, err)
	}

	mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(imagePath)))
	if mediaType == "" {
		mediaType = http.DetectContentType(content)
	}
	mediaType = strings.Split(mediaType, ";")[0]
	if !strings.HasPrefix(mediaType, "image/") {
		return "", false, fmt.Errorf("image %q has unsupported media type %q", dest, mediaType)
	}

	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(content), true, nil
}

// resolveLocalPath resolves a relative path from RelPath
// and rejects paths escaping Root, including through symlinks.
func resolveLocalPath(root, relPath, destination string) (string, error) {
	rootPath, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", root, err)
	}

	candidate := filepath.Join(
		rootPath,
		filepath.FromSlash(path.Dir(filepath.ToSlash(relPath))),
		filepath.FromSlash(destination),
	)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(rootPath, resolved)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("resolves above --root %q", root)
	}

	return resolved, nil
}

// rewriteDestination changes only relative destinations.
// Absolute and unusual destinations remain untouched because changing their semantics is unsafe.
func rewriteDestination(dest string, cfg RewriteConfig) (string, error) {
	if dest == "" || strings.HasPrefix(dest, "#") || strings.HasPrefix(dest, "//") {
		return dest, nil
	}

	u, err := url.Parse(dest)

	// An unparseable or scheme-qualified destination is left untouched,
	// not treated as a fatal error:
	// it is either already absolute or too unusual to safely rewrite.
	if err != nil || u.Scheme != "" {
		return dest, nil //nolint:nilerr // intentional: see comment above.
	}

	if cfg.BaseURL == "" {
		// Relative links remain portable when no publication URL is configured.
		return dest, nil
	}

	currentDir := path.Dir(filepath.ToSlash(cfg.RelPath))

	resolved := path.Join(currentDir, u.Path)
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return "", fmt.Errorf("link %q in %q resolves above --root %q", dest, cfg.RelPath, cfg.Root)
	}

	base, err := url.Parse(normalizeBaseURL(cfg.BaseURL))
	if err != nil {
		return "", fmt.Errorf("invalid --base-url %q: %w", cfg.BaseURL, err)
	}

	base.Path = path.Join(base.Path, resolved)
	base.RawQuery = u.RawQuery
	base.Fragment = u.Fragment

	return base.String(), nil
}

// normalizeBaseURL preserves a single trailing path separator.
func normalizeBaseURL(baseURL string) string {
	return strings.TrimSuffix(baseURL, "/") + "/"
}
