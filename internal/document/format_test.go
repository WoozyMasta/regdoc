// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	src := []byte("# Title\n")

	got, err := Render(src, FormatMarkdown)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if string(got) != string(src) {
		t.Fatalf("Render = %q, want %q", got, src)
	}
}

func TestRenderHTML(t *testing.T) {
	got, err := Render([]byte("# Title\n\n| A | B |\n| - | - |\n| 1 | 2 |\n"), FormatHTML)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, want := range []string{"<h1>Title</h1>", "<table>"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("Render = %q, want substring %q", got, want)
		}
	}
}

func TestRenderUnsupportedFormat(t *testing.T) {
	_, err := Render([]byte("content"), "text")
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func TestRenderHTMLRestoresEmbeddedImageDataURI(t *testing.T) {
	content := []byte("![Logo][regdoc-image-1]\n\n[regdoc-image-1]:data&colon;image/png;base64,iVBORw==\n")

	out, err := Render(content, FormatHTML)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !bytes.Contains(out, []byte(`src="data:image/png;base64,iVBORw=="`)) {
		t.Fatalf("expected data URI in HTML, got %q", out)
	}
}
