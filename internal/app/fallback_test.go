// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/woozymasta/regdoc/internal/document"
	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

// fakePublisher records every attempted publish and returns queued results
// in order (or the last one repeatedly, once exhausted).
type fakePublisher struct {
	results []error
	calls   []provider.Document
}

func (f *fakePublisher) Publish(_ context.Context, _ target.Target, doc provider.Document) error {
	f.calls = append(f.calls, doc)

	if len(f.results) == 0 {
		return nil
	}

	idx := len(f.calls) - 1
	if idx >= len(f.results) {
		idx = len(f.results) - 1
	}

	return f.results[idx]
}

// silentReporter discards diagnostics during publish tests.
func silentReporter() Reporter { return NewReporter(io.Discard, false, false) }

// publishForTest applies the CLI defaults used by payload-cut tests.
func publishForTest(pub *fakePublisher, parts []document.ProcessedPart, results ...error) error {
	pub.results = results

	return publish(
		context.Background(),
		pub,
		target.Target{},
		[]byte("# Header\n"),
		parts,
		"",
		FallbackCut,
		0,
		2,
		5,
		provider.Quay,
		document.FormatMarkdown,
		silentReporter(),
	)
}

func TestPublishFullPayloadAccepted(t *testing.T) {
	pub := &fakePublisher{}
	parts := []document.ProcessedPart{{Path: "a.md", Content: "a"}, {Path: "b.md", Content: "b"}}

	if err := publishForTest(pub, parts); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if len(pub.calls) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(pub.calls))
	}
}

func TestPublishCutsAtHeadingAfterPayloadTooLarge(t *testing.T) {
	pub := &fakePublisher{}
	parts := []document.ProcessedPart{{
		Path:    "README.md",
		Content: "# Intro\n\nKeep this.\n\n## Changelog\n\nDiscard this.",
	}}

	if err := publishForTest(pub, parts, provider.ErrPayloadTooLarge, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if len(pub.calls) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(pub.calls))
	}

	if strings.Contains(string(pub.calls[1].Content), "Changelog") {
		t.Fatalf("second attempt retained trailing section: %q", pub.calls[1].Content)
	}
}

func TestPublishDoesNotCutInsideFencedCode(t *testing.T) {
	pub := &fakePublisher{}
	parts := []document.ProcessedPart{{
		Path:    "README.md",
		Content: "intro\n\n```go\nfunc main() {\n\tprintln(\"value\")\n}\n```\n",
	}}

	if err := publishForTest(pub, parts, provider.ErrPayloadTooLarge, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if got := string(pub.calls[1].Content); strings.Contains(got, "```go") {
		t.Fatalf("second attempt retained a partial fenced block: %q", got)
	}
}

func TestPublishRendersHTMLAgainAfterPayloadTooLarge(t *testing.T) {
	pub := &fakePublisher{results: []error{provider.ErrPayloadTooLarge, nil}}
	parts := []document.ProcessedPart{{
		Path: "README.md",
		Content: "# Kept\n\n- one\n- two\n\n" +
			"| A | B |\n| - | - |\n| 1 | 2 |\n\n" +
			"```go\nfmt.Println(\"ok\")\n```\n\n" +
			"## Drop\n\n" + strings.Repeat("tail ", 2000),
	}}

	err := publish(
		context.Background(), pub, target.Target{}, nil, parts, "", FallbackCut,
		0, 2, 5, provider.Quay, document.FormatHTML, silentReporter(),
	)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if len(pub.calls) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(pub.calls))
	}

	got := string(pub.calls[1].Content)
	if strings.Contains(got, "Drop") {
		t.Fatalf("second attempt retained trailing section: %q", got)
	}
	for _, tag := range []string{"ul", "table", "pre", "code"} {
		if strings.Count(got, "<"+tag) != 1 || strings.Count(got, "</"+tag+">") != 1 {
			t.Fatalf("second attempt contains unbalanced <%s> tags: %q", tag, got)
		}
	}
}

func TestPublishStopsAfterConfiguredCutRetries(t *testing.T) {
	pub := &fakePublisher{results: []error{provider.ErrPayloadTooLarge}}
	parts := []document.ProcessedPart{{Path: "README.md", Content: strings.Repeat("content ", 100)}}

	err := publish(
		context.Background(), pub, target.Target{}, nil, parts, "", FallbackCut,
		0, 2, 2, provider.Quay, document.FormatMarkdown, silentReporter(),
	)
	if !errors.Is(err, provider.ErrPayloadTooLarge) {
		t.Fatalf("expected ErrPayloadTooLarge, got %v", err)
	}

	if len(pub.calls) != 3 {
		t.Fatalf("expected initial attempt plus 2 retries, got %d", len(pub.calls))
	}
}

func TestPublishFallbackNoneDoesNotRetry(t *testing.T) {
	pub := &fakePublisher{results: []error{provider.ErrPayloadTooLarge}}
	parts := []document.ProcessedPart{{Path: "README.md", Content: "content"}}

	err := publish(
		context.Background(), pub, target.Target{}, nil, parts, "", FallbackNone,
		0, 2, 5, provider.Quay, document.FormatMarkdown, silentReporter(),
	)
	if !errors.Is(err, provider.ErrPayloadTooLarge) {
		t.Fatalf("expected ErrPayloadTooLarge, got %v", err)
	}

	if len(pub.calls) != 1 {
		t.Fatalf("expected exactly 1 attempt, got %d", len(pub.calls))
	}
}

func TestPublishAppliesConfiguredBodyLimitBeforePublishing(t *testing.T) {
	pub := &fakePublisher{}
	parts := []document.ProcessedPart{{Path: "README.md", Content: "first\n\nsecond\n\nthird"}}

	if err := publish(
		context.Background(), pub, target.Target{}, nil, parts, "", FallbackCut,
		12, 2, 5, provider.Quay, document.FormatMarkdown, silentReporter(),
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if got := string(pub.calls[0].Content); got != "first" {
		t.Fatalf("Content = %q, want first", got)
	}
}

func TestPublishAppliesConfiguredBodyLimitBeforeRenderingHTML(t *testing.T) {
	pub := &fakePublisher{}
	parts := []document.ProcessedPart{{Path: "README.md", Content: strings.Repeat("content ", 100)}}

	if err := publish(
		context.Background(), pub, target.Target{}, nil, parts, "", FallbackCut,
		200, 2, 5, provider.Quay, document.FormatHTML, silentReporter(),
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	got := pub.calls[0].Content
	if len(got) > 200 {
		t.Fatalf("content size = %d, want at most 200", len(got))
	}
	if !strings.HasPrefix(string(got), "<p>") || !strings.HasSuffix(string(got), "</p>\n") {
		t.Fatalf("content is not a complete rendered paragraph: %q", got)
	}
}

func TestPublishAppliesDockerHubCharacterLimit(t *testing.T) {
	pub := &fakePublisher{}
	parts := []document.ProcessedPart{{Path: "README.md", Content: strings.Repeat("ё", dockerHubBodyLimit+1)}}

	if err := publish(
		context.Background(), pub, target.Target{}, nil, parts, "", FallbackCut,
		0, 2, 5, provider.DockerHub, document.FormatMarkdown, silentReporter(),
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if got := utf8.RuneCount(pub.calls[0].Content); got != dockerHubBodyLimit {
		t.Fatalf("rune count = %d, want %d", got, dockerHubBodyLimit)
	}
}

func TestPublishAppliesDockerHubCharacterLimitBeforeRenderingHTML(t *testing.T) {
	pub := &fakePublisher{}
	parts := []document.ProcessedPart{{Path: "README.md", Content: strings.Repeat("ё", dockerHubBodyLimit+1)}}

	if err := publish(
		context.Background(), pub, target.Target{}, nil, parts, "", FallbackCut,
		0, 2, 5, provider.DockerHub, document.FormatHTML, silentReporter(),
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	got := pub.calls[0].Content
	if size := utf8.RuneCount(got); size > dockerHubBodyLimit {
		t.Fatalf("rune count = %d, want at most %d", size, dockerHubBodyLimit)
	}
	if !strings.HasPrefix(string(got), "<p>") || !strings.HasSuffix(string(got), "</p>\n") {
		t.Fatalf("content is not a complete rendered paragraph: %q", got)
	}
}

func TestPublishDoesNotRetryOnAuthError(t *testing.T) {
	pub := &fakePublisher{results: []error{provider.ErrUnauthorized}}
	parts := []document.ProcessedPart{{Path: "README.md", Content: "content"}}

	err := publish(
		context.Background(), pub, target.Target{}, nil, parts, "", FallbackCut,
		0, 2, 5, provider.Quay, document.FormatMarkdown, silentReporter(),
	)
	if !errors.Is(err, provider.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}

	if len(pub.calls) != 1 {
		t.Fatalf("expected exactly 1 attempt on auth error, got %d", len(pub.calls))
	}
}

func TestPublishDoesNotRetryOnNetworkError(t *testing.T) {
	pub := &fakePublisher{results: []error{errors.New("connection reset")}}
	parts := []document.ProcessedPart{{Path: "README.md", Content: "content"}}

	err := publish(
		context.Background(), pub, target.Target{}, nil, parts, "", FallbackCut,
		0, 2, 5, provider.Quay, document.FormatMarkdown, silentReporter(),
	)
	if err == nil {
		t.Fatal("expected network error to propagate")
	}

	if len(pub.calls) != 1 {
		t.Fatalf("expected exactly 1 attempt on network error, got %d", len(pub.calls))
	}
}

func TestPublishPassesShortDescription(t *testing.T) {
	pub := &fakePublisher{}
	parts := []document.ProcessedPart{{Path: "README.md", Content: "content"}}

	if err := publish(
		context.Background(), pub, target.Target{}, nil, parts, "short", FallbackCut,
		0, 2, 5, provider.Quay, document.FormatMarkdown, silentReporter(),
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if pub.calls[0].ShortDescription != "short" {
		t.Fatalf("ShortDescription = %q", pub.calls[0].ShortDescription)
	}
}
