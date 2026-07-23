// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import "testing"

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
