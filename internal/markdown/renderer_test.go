// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package markdown

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// benchmarkCase describes a benchmark input corpus.
type benchmarkCase struct {
	name   string
	source []byte
}

// TestRendererCommonMark verifies CommonMark nodes render back to Markdown.
func TestRendererCommonMark(t *testing.T) {
	source := []byte("# Title\n\n* item\n\n> quote\n\n```go\nfmt.Println(\"ok\")\n```\n")
	md := goldmark.New(goldmark.WithRenderer(NewRenderer()))

	var output bytes.Buffer
	if err := md.Convert(source, &output); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	for _, want := range []string{"# Title", "* item", "> quote", "```go"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("output %q does not contain %q", output.String(), want)
		}
	}
}

// TestRendererFormattingRegression verifies stable formatting for supported nodes.
func TestRendererFormattingRegression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "paragraphs in blockquote",
			source: "> first\n>\n> second\n",
			want:   "> first\n>\n> second\n",
		},
		{
			name:   "ordered list keeps numbering",
			source: "3. third\n4. fourth\n",
			want:   "3. third\n4. fourth\n",
		},
		{
			name:   "unordered list keeps indentation",
			source: "* parent\n  * child\n",
			want:   "* parent\n  * child\n",
		},
		{
			name:   "html block keeps closure",
			source: "<details>\ncontent\n</details>\n",
			want:   "<details>\ncontent\n</details>\n",
		},
		{
			name:   "hard line break stays escaped",
			source: "line one\\\\\nline two\n",
			want:   "line one\\\\\nline two\n",
		},
		{
			name:   "inline html is preserved",
			source: "prefix <span>inline</span> suffix\n",
			want:   "prefix <span>inline</span> suffix\n",
		},
		{
			name:   "autolink stays bracketed",
			source: "<https://example.com>\n",
			want:   "<https://example.com>\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := convertMarkdown(t, []byte(tt.source))
			if got != tt.want {
				t.Fatalf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRendererGoldmark18ReferenceDefinition verifies reference links render inline.
func TestRendererGoldmark18ReferenceDefinition(t *testing.T) {
	source := []byte("[docs][ref]\n\n[ref]: ./guide.md\n")
	md := goldmark.New(goldmark.WithRenderer(NewRenderer()))

	var output bytes.Buffer
	if err := md.Convert(source, &output); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if got, want := output.String(), "[docs](./guide.md)\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

// TestRendererConsecutiveCodeSpans verifies adjacent code spans stay unchanged.
func TestRendererConsecutiveCodeSpans(t *testing.T) {
	source := []byte("`` `value` `` and `plain`\n")
	md := goldmark.New(goldmark.WithRenderer(NewRenderer()))

	var output bytes.Buffer
	if err := md.Convert(source, &output); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if got := output.String(); got != string(source) {
		t.Fatalf("output = %q, want %q", got, source)
	}
}

// TestAnalyzeCodeSpan verifies delimiter and padding analysis for code spans.
func TestAnalyzeCodeSpan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   codeSpanContext
	}{
		{
			name:   "plain content",
			source: "`value`\n",
			want:   codeSpanContext{backtickLength: 1},
		},
		{
			name:   "backtick wrapped content",
			source: "`` `value` ``\n",
			want:   codeSpanContext{backtickLength: 2, padSpace: true},
		},
		{
			name:   "normalized spaced content",
			source: "`` value ``\n",
			want:   codeSpanContext{backtickLength: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node, source := parseFirstCodeSpan(t, tt.source)
			got := analyzeCodeSpan(node, source)
			if got != tt.want {
				t.Fatalf("analyzeCodeSpan(%q) = %+v, want %+v", tt.source, got, tt.want)
			}
		})
	}
}

// TestRendererReusableAcrossGoroutines verifies Render does not keep shared mutable state.
func TestRendererReusableAcrossGoroutines(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer()
	source := []byte("# Title\n\n* item\n\n> quote\n")
	root := parseMarkdown(t, source)

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var output bytes.Buffer
			if err := renderer.Render(&output, source, root); err != nil {
				t.Errorf("Render: %v", err)
				return
			}

			if got, want := output.String(), "# Title\n\n* item\n\n> quote\n"; got != want {
				t.Errorf("output = %q, want %q", got, want)
			}
		}()
	}
	wg.Wait()
}

// TestMarkdownWriterTrimTrailingSpace verifies rendered lines drop trailing space.
func TestMarkdownWriterTrimTrailingSpace(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	writer := newMarkdownWriter(&output)
	writer.writeBytes([]byte("value   \n"))
	if err := writer.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got, want := output.String(), "value\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

// TestMarkdownWriterScopedPrefixes verifies prefix ranges apply to the intended lines.
func TestMarkdownWriterScopedPrefixes(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	writer := newMarkdownWriter(&output)
	writer.pushPrefix([]byte("* "), 0, 0)
	writer.pushPrefix([]byte("  "), 1)
	writer.writeBytes([]byte("first\nsecond\nthird\n"))
	if err := writer.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got, want := output.String(), "* first\n  second\n  third\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

// BenchmarkRenderer measures full-document render throughput across key flows.
func BenchmarkRenderer(b *testing.B) {
	for _, bench := range benchmarkCases() {
		b.Run(bench.name, func(b *testing.B) {
			md := goldmark.New(goldmark.WithRenderer(NewRenderer()))
			b.ReportAllocs()
			b.SetBytes(int64(len(bench.source)))

			for range b.N {
				var output bytes.Buffer
				if err := md.Convert(bench.source, &output); err != nil {
					b.Fatalf("Convert: %v", err)
				}
			}
		})
	}
}

// BenchmarkRendererRenderOnly measures render throughput without parse cost.
func BenchmarkRendererRenderOnly(b *testing.B) {
	for _, bench := range benchmarkCases() {
		b.Run(bench.name, func(b *testing.B) {
			renderer := NewRenderer()
			root := parseMarkdown(b, bench.source)
			b.ReportAllocs()
			b.SetBytes(int64(len(bench.source)))

			for range b.N {
				var output bytes.Buffer
				if err := renderer.Render(&output, bench.source, root); err != nil {
					b.Fatalf("Render: %v", err)
				}
			}
		})
	}
}

// benchmarkCases returns benchmark corpora for parser and renderer paths.
func benchmarkCases() []benchmarkCase {
	return []benchmarkCase{
		{
			name: "commonmark_mixed",
			source: []byte(strings.Join([]string{
				"# Title",
				"",
				"Paragraph with `inline` code and <https://example.com>.",
				"",
				"> quoted",
				">",
				"> second paragraph",
				"",
				"* item one",
				"* item two",
				"",
				"```go",
				`fmt.Println("ok")`,
				"```",
				"",
			}, "\n")),
		},
		{
			name:   "nested_lists",
			source: []byte(buildNestedListDocument(80)),
		},
		{
			name:   "codespan_heavy",
			source: []byte(buildCodeSpanDocument(200)),
		},
	}
}

// FuzzRendererRoundTrip verifies supported Markdown never panics and stays parseable.
func FuzzRendererRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("# Title\n\nParagraph\n"),
		[]byte("> quote\n>\n> second\n"),
		[]byte("`` `value` `` and `plain`\n"),
		[]byte("* item\n  * nested\n"),
		[]byte("[docs][ref]\n\n[ref]: ./guide.md\n"),
		[]byte("<details>\ncontent\n</details>\n"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, source []byte) {
		md := goldmark.New(goldmark.WithRenderer(NewRenderer()))

		var output bytes.Buffer
		if err := md.Convert(source, &output); err != nil {
			t.Skip()
		}

		var reparsed bytes.Buffer
		if err := md.Convert(output.Bytes(), &reparsed); err != nil {
			t.Fatalf("reparse rendered output: %v\nsource=%q\noutput=%q", err, source, output.Bytes())
		}
	})
}

// convertMarkdown renders source with the package renderer.
func convertMarkdown(t *testing.T, source []byte) string {
	t.Helper()

	md := goldmark.New(goldmark.WithRenderer(NewRenderer()))
	var output bytes.Buffer
	if err := md.Convert(source, &output); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	return output.String()
}

// parseFirstCodeSpan parses source and returns the first code span node.
func parseFirstCodeSpan(t testing.TB, source string) (ast.Node, []byte) {
	t.Helper()

	sourceBytes := []byte(source)
	root := parseMarkdown(t, sourceBytes)
	var codeSpan ast.Node
	if err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && node.Kind() == ast.KindCodeSpan {
			codeSpan = node
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if codeSpan == nil {
		t.Fatalf("no code span found in %q", source)
	}

	return codeSpan, sourceBytes
}

// parseMarkdown parses Markdown source into a Goldmark AST.
func parseMarkdown(t testing.TB, source []byte) ast.Node {
	t.Helper()

	md := goldmark.New()
	return md.Parser().Parse(text.NewReader(source))
}

// buildNestedListDocument returns a document with repeated nested list content.
func buildNestedListDocument(items int) string {
	var builder strings.Builder
	for i := range items {
		builder.WriteString(fmt.Sprintf("%d. parent %d\n", i+1, i+1))
		builder.WriteString("   * child a\n")
		builder.WriteString("   * child b\n")
	}

	return builder.String()
}

// buildCodeSpanDocument returns a document with many code span edge cases.
func buildCodeSpanDocument(lines int) string {
	var builder strings.Builder
	for i := range lines {
		builder.WriteString("Prefix `` `value` `` and `plain` and `` spaced `` suffix ")
		builder.WriteString(fmt.Sprintf("%d\n", i))
	}

	return builder.String()
}
