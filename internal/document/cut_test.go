// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"strings"
	"testing"
)

var benchmarkCutResult []byte

func TestCutPrefersEligibleHeading(t *testing.T) {
	content := []byte("# Intro\n\nText\n\n### Detail\n\nMore\n\n## Changelog\n\nOld releases")
	limit := len("# Intro\n\nText\n\n### Detail\n\nMore\n\n## Changelog\n")

	if got := string(Cut(content, limit, 2)); got != "# Intro\n\nText\n\n### Detail\n\nMore\n\n" {
		t.Fatalf("Cut = %q", got)
	}
}

func TestCutFallsBackToBlankLine(t *testing.T) {
	content := []byte("first paragraph\n\nsecond paragraph")

	if got := string(Cut(content, 22, 2)); got != "first paragraph" {
		t.Fatalf("Cut = %q", got)
	}
}

func TestCutPreservesUTF8(t *testing.T) {
	content := []byte("one ё two")

	if got := string(Cut(content, 5, 2)); got != "one " {
		t.Fatalf("Cut = %q", got)
	}
}

func TestCutFallsBackToCompleteLine(t *testing.T) {
	content := []byte("first line\nsecond line")

	if got := string(Cut(content, 18, 2)); got != "first line" {
		t.Fatalf("Cut = %q, want first line", got)
	}
}

func TestLimitRunes(t *testing.T) {
	if got := string(LimitRunes([]byte("abёcd"), 3)); got != "abё" {
		t.Fatalf("LimitRunes = %q", got)
	}
}

func TestCutRecognizesHTMLHeading(t *testing.T) {
	content := []byte("<h1>Intro</h1>\n<p>Text</p>\n<h2>Changelog</h2>\n<p>Old releases</p>\n")
	limit := len("<h1>Intro</h1>\n<p>Text</p>\n<h2>Changelog</h2>\n")

	if got := string(Cut(content, limit, 2)); got != "<h1>Intro</h1>\n<p>Text</p>\n" {
		t.Fatalf("Cut = %q", got)
	}
}

func TestCutBeforeOpenBacktickFence(t *testing.T) {
	content := []byte("intro\n\n```go\n# not a heading\n\nvalue\n```\nafter\n")
	limit := len("intro\n\n```go\n# not a heading\n\nval")

	if got := string(Cut(content, limit, 2)); got != "intro" {
		t.Fatalf("Cut = %q, want intro", got)
	}
}

func TestCutBeforeOpenTildeFence(t *testing.T) {
	content := []byte("intro\n\n~~~~text\nvalue\n~~~~\nafter\n")
	limit := len("intro\n\n~~~~text\nval")

	if got := string(Cut(content, limit, 2)); got != "intro" {
		t.Fatalf("Cut = %q, want intro", got)
	}
}

func TestCutInsideFenceAtDocumentStartReturnsEmpty(t *testing.T) {
	content := []byte("```text\nvalue\n```\n")
	limit := len("```text\nval")

	if got := Cut(content, limit, 2); len(got) != 0 {
		t.Fatalf("Cut = %q, want empty document", got)
	}
}

func TestCutIgnoresHeadingInsideClosedFence(t *testing.T) {
	content := []byte("# Intro\n\n```md\n## not a section\n```\n\nparagraph that is cut")
	limit := len("# Intro\n\n```md\n## not a section\n```\n\nparagraph")

	want := "# Intro\n\n```md\n## not a section\n```"
	if got := string(Cut(content, limit, 2)); got != want {
		t.Fatalf("Cut = %q, want %q", got, want)
	}
}

func BenchmarkCut(b *testing.B) {
	plain := []byte(strings.Repeat("## Section\n\nParagraph content.\n\n", 8*1024))
	fenced := []byte("# Title\n\n```text\n" + strings.Repeat("code line\n", 32*1024) + "```\n")
	longLine := []byte(strings.Repeat("x", 256*1024))

	benchmarks := []struct {
		name    string
		content []byte
		limit   int
	}{
		{name: "plain_markdown", content: plain, limit: len(plain) * 3 / 4},
		{name: "inside_fence", content: fenced, limit: len(fenced) * 3 / 4},
		{name: "long_line", content: longLine, limit: len(longLine) * 3 / 4},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(benchmark.limit))
			for range b.N {
				benchmarkCutResult = Cut(benchmark.content, benchmark.limit, 2)
			}
		})
	}
}
