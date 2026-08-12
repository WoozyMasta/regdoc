// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package markdown renders a Goldmark AST back to Markdown.
//
// It intentionally supports only Goldmark's built-in CommonMark nodes used by regdoc.
// Unknown nodes are traversed without rendering their wrappers,
// which keeps the renderer forward-compatible with new Goldmark node kinds.
package markdown

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
)

// Ensure Renderer implements renderer.Renderer.
var _ renderer.Renderer = (*Renderer)(nil)

// Renderer renders Goldmark's built-in CommonMark nodes as Markdown.
type Renderer struct {
	// nodeRenderers maps node kinds to rendering callbacks.
	nodeRenderers map[ast.NodeKind]nodeRenderer
}

// renderContext stores state shared across a single render pass.
type renderContext struct {
	writer              *markdownWriter   // writer emits the rendered Markdown output.
	source              []byte            // source contains the original document bytes.
	lists               []listContext     // lists tracks nested list rendering state.
	embeddedImages      map[string]string // embeddedImages maps data URIs to reference labels.
	embeddedDefinitions []string          // embeddedDefinitions stores generated reference definitions.
	codeSpan            codeSpanContext   // codeSpan stores temporary inline code rendering state.
}

// listContext stores numbering and marker state for a list level.
type listContext struct {
	list   *ast.List // list is the current Goldmark list node.
	number int       // number is the next ordered list number to render.
}

// codeSpanContext stores delimiter and padding state for code spans.
type codeSpanContext struct {
	backtickLength int  // backtickLength is the chosen delimiter width.
	padSpace       bool // padSpace reports whether the code span needs outer spaces.
}

// nodeRenderer renders a single node during AST traversal.
type nodeRenderer func(*renderContext, ast.Node, bool) ast.WalkStatus

// NewRenderer returns a Markdown renderer with regdoc's fixed formatting.
func NewRenderer() *Renderer {
	r := &Renderer{}
	r.nodeRenderers = map[ast.NodeKind]nodeRenderer{
		ast.KindDocument:        chainRenderers(r.renderDocument, r.renderBlockSeparator),
		ast.KindHeading:         chainRenderers(r.renderBlockSeparator, r.renderHeading),
		ast.KindBlockquote:      chainRenderers(r.renderBlockSeparator, r.renderBlockquote),
		ast.KindCodeBlock:       chainRenderers(r.renderBlockSeparator, r.renderCodeBlock),
		ast.KindFencedCodeBlock: chainRenderers(r.renderBlockSeparator, r.renderFencedCodeBlock),
		ast.KindHTMLBlock:       chainRenderers(r.renderBlockSeparator, r.renderHTMLBlock),
		ast.KindList:            chainRenderers(r.renderBlockSeparator, r.renderList),
		ast.KindListItem:        chainRenderers(r.renderBlockSeparator, r.renderListItem),
		ast.KindParagraph:       chainRenderers(r.renderBlockSeparator, r.renderParagraph),
		ast.KindTextBlock:       r.renderBlockSeparator,
		ast.KindThematicBreak:   chainRenderers(r.renderBlockSeparator, r.renderThematicBreak),
		ast.KindAutoLink:        r.renderAutoLink,
		ast.KindCodeSpan:        r.renderCodeSpan,
		ast.KindEmphasis:        r.renderEmphasis,
		ast.KindImage:           r.renderImage,
		ast.KindLink:            r.renderLink,
		ast.KindRawHTML:         r.renderRawHTML,
		ast.KindText:            r.renderText,
		ast.KindString:          r.renderString,
	}

	return r
}

// renderDocument appends reference definitions for embedded images.
func (r *Renderer) renderDocument(context *renderContext, _ ast.Node, entering bool) ast.WalkStatus {
	if entering || len(context.embeddedDefinitions) == 0 {
		return ast.WalkContinue
	}

	context.writer.endLine()
	for _, definition := range context.embeddedDefinitions {
		context.writer.writeLine([]byte(definition))
	}

	return ast.WalkContinue
}

// AddOptions implements renderer.Renderer.
// The internal renderer deliberately has no configurable options.
func (r *Renderer) AddOptions(_ ...renderer.Option) {}

// Render implements renderer.Renderer.
func (r *Renderer) Render(w io.Writer, source []byte, root ast.Node) error {
	context := newRenderContext(w, source)

	err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		fn := r.nodeRenderers[node.Kind()]
		if fn == nil {
			return ast.WalkContinue, context.writer.errorState()
		}

		return fn(&context, node, entering), context.writer.errorState()
	})
	if err != nil {
		return err
	}

	return context.writer.flush()
}

// renderBlockSeparator flushes block boundaries and preserves blank lines.
func (r *Renderer) renderBlockSeparator(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	if entering {
		if node.PreviousSibling() != nil && node.HasBlankPreviousLines() {
			context.writer.endLine()
		}
	} else {
		context.writer.flushLine()
	}

	return ast.WalkContinue
}

// renderParagraph keeps paragraph separation inside blockquotes.
func (r *Renderer) renderParagraph(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	if entering {
		previous := node.PreviousSibling()
		if previous != nil && ast.IsParagraph(previous) &&
			node.Parent().Kind() == ast.KindBlockquote &&
			!node.HasBlankPreviousLines() {
			context.writer.endLine()
		}
	}

	return ast.WalkContinue
}

// renderAutoLink renders an autolink using angle-bracket syntax.
func (r *Renderer) renderAutoLink(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	link := node.(*ast.AutoLink)
	if entering {
		context.writer.writeBytes([]byte("<"))
		context.writer.writeBytes(link.URL(context.source))
	} else {
		context.writer.writeBytes([]byte(">"))
	}

	return ast.WalkContinue
}

// renderBlockquote applies the blockquote line prefix.
func (r *Renderer) renderBlockquote(context *renderContext, _ ast.Node, entering bool) ast.WalkStatus {
	if entering {
		context.writer.pushPrefix([]byte("> "))
	} else {
		context.writer.popPrefix()
	}

	return ast.WalkContinue
}

// renderHeading renders ATX headings with the original level.
func (r *Renderer) renderHeading(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	heading := node.(*ast.Heading)
	if entering {
		context.writer.writeBytes(bytes.Repeat([]byte("#"), heading.Level))
		if heading.HasChildren() {
			context.writer.writeBytes([]byte(" "))
		}
	}

	return ast.WalkContinue
}

// renderThematicBreak renders a CommonMark thematic break.
func (r *Renderer) renderThematicBreak(context *renderContext, _ ast.Node, entering bool) ast.WalkStatus {
	if entering {
		context.writer.writeBytes([]byte("---"))
	}

	return ast.WalkContinue
}

// renderCodeBlock renders an indented code block.
func (r *Renderer) renderCodeBlock(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	if entering {
		context.writer.pushPrefix([]byte("    "))
		context.writer.preserveTrailingWhitespace = true
		r.renderLines(context, node)
		context.writer.preserveTrailingWhitespace = false
	} else {
		context.writer.popPrefix()
	}

	return ast.WalkContinue
}

// renderFencedCodeBlock renders a fenced code block and its info string.
func (r *Renderer) renderFencedCodeBlock(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	block := node.(*ast.FencedCodeBlock)
	fence := fencedCodeDelimiter(block, context.source)
	context.writer.writeBytes(fence)
	if entering {
		if block.Info != nil {
			context.writer.writeBytes(block.Info.Value(context.source))
		}
		context.writer.flushLine()
		context.writer.preserveTrailingWhitespace = true
		r.renderLines(context, node)
		context.writer.preserveTrailingWhitespace = false
	}

	return ast.WalkContinue
}

// renderHTMLBlock renders an HTML block including its closing line.
func (r *Renderer) renderHTMLBlock(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	block := node.(*ast.HTMLBlock)
	if entering {
		r.renderLines(context, node)
	} else if block.HasClosure() {
		context.writer.writeLine(block.ClosureLine.Value(context.source))
	}

	return ast.WalkContinue
}

// renderList tracks list state for nested list items.
func (r *Renderer) renderList(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	if entering {
		list := node.(*ast.List)
		context.lists = append(context.lists, listContext{list: list, number: list.Start})
	} else {
		context.lists = context.lists[:len(context.lists)-1]
	}

	return ast.WalkContinue
}

// renderListItem renders the marker and indentation for a list item.
func (r *Renderer) renderListItem(context *renderContext, _ ast.Node, entering bool) ast.WalkStatus {
	if entering {
		list := &context.lists[len(context.lists)-1]
		var prefix []byte
		if list.list.IsOrdered() {
			prefix = fmt.Append(prefix, list.number)
			list.number++
		}
		prefix = append(prefix, list.list.Marker, ' ')
		context.writer.pushPrefix(prefix, 0, 0)
		context.writer.pushPrefix(bytes.Repeat([]byte(" "), len(prefix)), 1)
	} else {
		context.writer.popPrefix()
		context.writer.popPrefix()
	}

	return ast.WalkContinue
}

// renderRawHTML writes inline raw HTML segments as-is.
func (r *Renderer) renderRawHTML(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	if entering {
		r.renderSegments(context, node.(*ast.RawHTML).Segments, false)
	}

	return ast.WalkContinue
}

// renderText writes plain text and preserves explicit line breaks.
func (r *Renderer) renderText(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	if !entering {
		return ast.WalkContinue
	}

	value := node.(*ast.Text)
	context.writer.writeBytes(value.Value(context.source))
	if value.SoftLineBreak() {
		context.writer.endLine()
	} else if value.HardLineBreak() {
		_, _ = context.writer.writeRune('\\')
		context.writer.endLine()
	}

	return ast.WalkContinue
}

// renderString writes literal string node contents.
func (r *Renderer) renderString(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	if entering {
		context.writer.writeBytes(node.(*ast.String).Value)
	}

	return ast.WalkContinue
}

// renderSegments writes text segments either inline or one line at a time.
func (r *Renderer) renderSegments(context *renderContext, segments *text.Segments, asLines bool) {
	for i := range segments.Len() {
		segment := segments.At(i)
		context.writer.writeBytes(segment.Value(context.source))
		if asLines {
			context.writer.flushLine()
		}
	}
}

// renderLines renders node lines and flushes each line separately.
func (r *Renderer) renderLines(context *renderContext, node ast.Node) {
	r.renderSegments(context, node.Lines(), true)
}

// renderLink renders a Markdown link.
func (r *Renderer) renderLink(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	link := node.(*ast.Link)
	return r.renderLinkCommon(context, link.Title, link.Destination, entering)
}

// renderImage renders a Markdown image.
func (r *Renderer) renderImage(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	image := node.(*ast.Image)
	dataURI, embedded := embeddedImageDataURI(string(image.Destination))
	if embedded {
		imageKey := dataURI + "\x00" + string(image.Title)
		label, ok := context.embeddedImages[imageKey]
		if !ok {
			label = fmt.Sprintf("regdoc-image-%d", len(context.embeddedImages)+1)
			context.embeddedImages[imageKey] = label
			definition := "[" + label + "]:data&colon;" + strings.TrimPrefix(dataURI, "data:")
			if len(image.Title) > 0 {
				definition += ` "` + string(image.Title) + `"`
			}
			context.embeddedDefinitions = append(context.embeddedDefinitions, definition)
		}

		if entering {
			context.writer.writeBytes([]byte("!["))
		} else {
			context.writer.writeBytes([]byte("][" + label + "]"))
		}

		return ast.WalkContinue
	}

	if entering {
		context.writer.writeBytes([]byte("!"))
	}

	return r.renderLinkCommon(context, image.Title, image.Destination, entering)
}

// fencedCodeDelimiter returns a fence that cannot close inside the block.
func fencedCodeDelimiter(block *ast.FencedCodeBlock, source []byte) []byte {
	marker := byte('`')
	if block.Info != nil && bytes.ContainsRune(block.Info.Value(source), '`') {
		marker = '~'
	}

	longestRun := 0
	for i := range block.Lines().Len() {
		segment := block.Lines().At(i)
		line := segment.Value(source)
		run := 0
		for _, value := range line {
			if value == marker {
				run++
				longestRun = max(longestRun, run)
			} else {
				run = 0
			}
		}
	}

	return bytes.Repeat([]byte{marker}, max(3, longestRun+1))
}

// embeddedImageDataURI recognizes raw and entity-escaped inline base64 images.
func embeddedImageDataURI(destination string) (string, bool) {
	switch {
	case strings.HasPrefix(destination, "data:image/"):
	case strings.HasPrefix(destination, "data&colon;image/"):
		destination = "data:" + strings.TrimPrefix(destination, "data&colon;")
	default:
		return "", false
	}

	_, _, ok := strings.Cut(destination, ";base64,")
	return destination, ok
}

// renderLinkCommon renders shared Markdown link and image syntax.
func (r *Renderer) renderLinkCommon(context *renderContext, title, destination []byte, entering bool) ast.WalkStatus {
	if entering {
		context.writer.writeBytes([]byte("["))
	} else {
		context.writer.writeBytes([]byte("]("))
		context.writer.writeBytes(destination)
		if len(title) > 0 {
			context.writer.writeBytes([]byte(" \""))
			context.writer.writeBytes(title)
			context.writer.writeBytes([]byte("\""))
		}
		context.writer.writeBytes([]byte(")"))
	}

	return ast.WalkContinue
}

// renderCodeSpan renders an inline code span with safe backtick padding.
func (r *Renderer) renderCodeSpan(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
	if entering {
		context.codeSpan = analyzeCodeSpan(node, context.source)
		context.writer.writeBytes(bytes.Repeat([]byte("`"), context.codeSpan.backtickLength))
		if context.codeSpan.padSpace {
			context.writer.writeBytes([]byte(" "))
		}
	} else {
		if context.codeSpan.padSpace {
			context.writer.writeBytes([]byte(" "))
		}
		context.writer.writeBytes(bytes.Repeat([]byte("`"), context.codeSpan.backtickLength))
	}

	return ast.WalkContinue
}

// renderEmphasis renders emphasis markers for the current emphasis level.
func (r *Renderer) renderEmphasis(context *renderContext, node ast.Node, _ bool) ast.WalkStatus {
	level := node.(*ast.Emphasis).Level
	context.writer.writeBytes(bytes.Repeat([]byte{'*'}, level))
	return ast.WalkContinue
}

// chainRenderers combines multiple renderers into a single renderer.
func chainRenderers(renderers ...nodeRenderer) nodeRenderer {
	return func(context *renderContext, node ast.Node, entering bool) ast.WalkStatus {
		status := ast.WalkContinue
		for i := range renderers {
			if !entering {
				i = len(renderers) - 1 - i
			}
			status = renderers[i](context, node, entering)
		}

		return status
	}
}

// analyzeCodeSpan returns delimiter and padding requirements for code span content.
func analyzeCodeSpan(node ast.Node, source []byte) codeSpanContext {
	codeSpan := codeSpanContext{backtickLength: 1}
	backtickRun := 0
	maxBacktickRun := 0
	allSpace := true
	hasContent := false
	var first rune
	var last rune

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		segment := child.(*ast.Text).Segment.Value(source)
		for len(segment) > 0 {
			value, size := utf8.DecodeRune(segment)
			segment = segment[size:]

			if !hasContent {
				first = value
				hasContent = true
			}
			last = value

			if value == '`' {
				backtickRun++
				if backtickRun > maxBacktickRun {
					maxBacktickRun = backtickRun
				}
			} else {
				backtickRun = 0
			}

			if !unicode.IsSpace(value) {
				allSpace = false
			}
		}
	}

	codeSpan.backtickLength = maxBacktickRun + 1
	codeSpan.padSpace = hasContent && ((unicode.IsSpace(first) &&
		unicode.IsSpace(last) &&
		!allSpace) || first == '`' || last == '`')

	return codeSpan
}

// newRenderContext builds per-render state for a renderer invocation.
func newRenderContext(writer io.Writer, source []byte) renderContext {
	return renderContext{
		writer:         newMarkdownWriter(writer),
		source:         source,
		embeddedImages: make(map[string]string),
	}
}
