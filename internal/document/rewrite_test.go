// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rewriteWithBase rewrites src with a stable base URL for assertions.
func rewriteWithBase(t *testing.T, relPath, src string) string {
	t.Helper()

	out, err := Rewrite([]byte(src), RewriteConfig{
		Root:          "root",
		RelPath:       relPath,
		BaseURL:       "https://git.example/project/-/raw/main/",
		StripComments: true,
	})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	return string(out)
}

func TestRewriteLinks(t *testing.T) {
	cases := []struct {
		name    string
		relPath string
		src     string
		want    string
	}{
		{
			name:    "inline link",
			relPath: "README.md",
			src:     "[docs](./docs/guide.md)\n",
			want:    "https://git.example/project/-/raw/main/docs/guide.md",
		},
		{
			name:    "image nested source directory",
			relPath: "docs/guide.md",
			src:     "![Logo](../images/logo.png)\n",
			want:    "https://git.example/project/-/raw/main/images/logo.png",
		},
		{
			name:    "reference link collapses to inline",
			relPath: "README.md",
			src:     "[docs][ref]\n\n[ref]: ./docs/guide.md\n",
			want:    "https://git.example/project/-/raw/main/docs/guide.md",
		},
		{
			name:    "query",
			relPath: "README.md",
			src:     "[docs](./docs/guide.md?raw=1)\n",
			want:    "https://git.example/project/-/raw/main/docs/guide.md?raw=1",
		},
		{
			name:    "fragment",
			relPath: "README.md",
			src:     "[docs](./docs/guide.md#section)\n",
			want:    "https://git.example/project/-/raw/main/docs/guide.md#section",
		},
		{
			name:    "query and fragment",
			relPath: "README.md",
			src:     "[docs](./docs/guide.md?raw=1#section)\n",
			want:    "https://git.example/project/-/raw/main/docs/guide.md?raw=1#section",
		},
		{
			name:    "url escaped destination",
			relPath: "README.md",
			src:     "[docs](./docs/my%20guide.md)\n",
			want:    "https://git.example/project/-/raw/main/docs/my%20guide.md",
		},
		{
			name:    "anchor only is untouched",
			relPath: "README.md",
			src:     "[toc](#описание)\n",
			want:    "(#описание)",
		},
		{
			name:    "absolute http untouched",
			relPath: "README.md",
			src:     "[x](https://example.com/a)\n",
			want:    "(https://example.com/a)",
		},
		{
			name:    "protocol relative untouched",
			relPath: "README.md",
			src:     "[x](//example.com/a)\n",
			want:    "(//example.com/a)",
		},
		{
			name:    "mailto untouched",
			relPath: "README.md",
			src:     "[x](mailto:a@example.com)\n",
			want:    "(mailto:a@example.com)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteWithBase(t, tc.relPath, tc.src)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("got %q, want substring %q", got, tc.want)
			}
		})
	}
}

func TestRewriteNoBaseURLLeavesRelativeUntouched(t *testing.T) {
	out, err := Rewrite([]byte("[docs](./docs/guide.md)\n"), RewriteConfig{
		Root:    "root",
		RelPath: "README.md",
	})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	if !strings.Contains(string(out), "(docs/guide.md)") && !strings.Contains(string(out), "(./docs/guide.md)") {
		t.Fatalf("expected relative destination untouched, got %q", out)
	}
}

func TestRewriteEscapeAboveRootIsError(t *testing.T) {
	_, err := Rewrite([]byte("[x](../../outside.md)\n"), RewriteConfig{
		Root:    "root",
		RelPath: "README.md",
		BaseURL: "https://git.example/project/-/raw/main/",
	})
	if err == nil {
		t.Fatal("expected error for link resolving above root")
	}
}

func TestRewriteCodeUntouched(t *testing.T) {
	src := "```\n[x](./should-not-move.md)\n```\n\n`[y](./also-not.md)`\n"

	out := rewriteWithBase(t, "README.md", src)

	if !strings.Contains(out, "./should-not-move.md") || !strings.Contains(out, "./also-not.md") {
		t.Fatalf("expected code content untouched, got %q", out)
	}
}

func TestRewriteRawHTMLLinksUntouched(t *testing.T) {
	src := `<a href="./page.md">link</a>` + "\n"

	out := rewriteWithBase(t, "README.md", src)

	if !strings.Contains(out, `href="./page.md"`) {
		t.Fatalf("expected raw html href untouched, got %q", out)
	}
}

func TestRewriteEmptyDocument(t *testing.T) {
	out, err := Rewrite([]byte(""), RewriteConfig{Root: "root", RelPath: "README.md"})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	if len(bytesTrim(out)) != 0 {
		t.Fatalf("expected empty output, got %q", out)
	}
}

// bytesTrim removes surrounding whitespace from test output.
func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func TestRewriteUnicode(t *testing.T) {
	out := rewriteWithBase(t, "README.md", "# Заголовок\n\nПривет, мир! 世界\n")

	if !strings.Contains(out, "Заголовок") || !strings.Contains(out, "世界") {
		t.Fatalf("expected unicode preserved, got %q", out)
	}
}

func TestRewriteCRLFInput(t *testing.T) {
	out, err := Rewrite([]byte("# Title\r\n\r\nBody text.\r\n"), RewriteConfig{Root: "root", RelPath: "README.md"})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	if !strings.Contains(string(out), "Title") || !strings.Contains(string(out), "Body text.") {
		t.Fatalf("expected content preserved across CRLF input, got %q", out)
	}
}

func TestRewriteTrailingNewline(t *testing.T) {
	out, err := Rewrite([]byte("# Title\n"), RewriteConfig{Root: "root", RelPath: "README.md"})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	if !strings.HasSuffix(string(out), "\n") || strings.HasSuffix(string(out), "\n\n") {
		t.Fatalf("expected exactly one trailing newline, got %q", out)
	}
}

func TestRewriteStripComments(t *testing.T) {
	src := "<!-- markdownlint-disable MD013 -->\n\n" +
		"# Title\n\n" +
		"Text with an inline <!-- note --> comment.\n\n" +
		"```html\n<!-- keep me, this is code -->\n```\n"

	out := rewriteWithBase(t, "README.md", src)

	if strings.Contains(out, "markdownlint-disable") {
		t.Fatalf("expected block comment stripped, got %q", out)
	}

	if strings.Contains(out, "<!-- note -->") {
		t.Fatalf("expected inline comment stripped, got %q", out)
	}

	if !strings.Contains(out, "keep me, this is code") {
		t.Fatalf("expected comment inside fenced code block preserved, got %q", out)
	}
}

func TestRewriteKeepComments(t *testing.T) {
	out, err := Rewrite([]byte("<!-- keep-me -->\n\n# Title\n"), RewriteConfig{
		Root:          "root",
		RelPath:       "README.md",
		StripComments: false,
	})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	if !strings.Contains(string(out), "keep-me") {
		t.Fatalf("expected comment preserved when StripComments is false, got %q", out)
	}
}
func TestRewriteEmbedLocalImage(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "images", "logo.png")
	if err := os.MkdirAll(filepath.Dir(imagePath), 0o755); err != nil {
		t.Fatalf("create image directory: %v", err)
	}
	if err := os.WriteFile(imagePath, []byte{0x89, 'P', 'N', 'G'}, 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	out, err := Rewrite([]byte("![Logo](images/logo.png)\n"), RewriteConfig{
		Root:        dir,
		RelPath:     "README.md",
		EmbedImages: true,
	})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	if strings.Contains(string(out), "(data:image/png;base64,") {
		t.Fatalf("expected embedded image reference, got %q", out)
	}

	for _, want := range []string{
		"![Logo][regdoc-image-1]",
		"[regdoc-image-1]:data&colon;image/png;base64,iVBORw==",
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("expected %q in %q", want, out)
		}
	}
}

func TestRewriteEmbedImagesLeavesExternalURL(t *testing.T) {
	out, err := Rewrite([]byte("![Logo](https://example.com/logo.png)\n"), RewriteConfig{
		Root:        t.TempDir(),
		RelPath:     "README.md",
		EmbedImages: true,
	})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	if !strings.Contains(string(out), "https://example.com/logo.png") {
		t.Fatalf("expected external URL untouched, got %q", out)
	}
}

func TestRewriteEmbedImageRejectsPathOutsideRoot(t *testing.T) {
	_, err := Rewrite([]byte("![Logo](../../outside.png)\n"), RewriteConfig{
		Root:        t.TempDir(),
		RelPath:     "docs/README.md",
		EmbedImages: true,
	})
	if err == nil {
		t.Fatal("expected image path outside root to fail")
	}
}
