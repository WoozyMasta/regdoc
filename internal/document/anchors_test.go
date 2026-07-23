// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"strings"
	"testing"

	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

func TestFinalizeAnchorsQuay(t *testing.T) {
	tgt := target.Target{Registry: "registry.example.com", Repository: "group/image"}

	out, err := FinalizeAnchors([]byte("[Описание](#описание)\n"), provider.Quay, tgt)
	if err != nil {
		t.Fatalf("FinalizeAnchors: %v", err)
	}

	want := "https://registry.example.com/repository/group/image#описание"
	if !strings.Contains(string(out), want) {
		t.Fatalf("got %q, want substring %q", out, want)
	}
}

func TestFinalizeAnchorsDockerHub(t *testing.T) {
	tgt := target.Target{Registry: "index.docker.io", Repository: "user/image"}

	out, err := FinalizeAnchors([]byte("[toc](#toc)\n"), provider.DockerHub, tgt)
	if err != nil {
		t.Fatalf("FinalizeAnchors: %v", err)
	}

	want := "https://hub.docker.com/r/user/image#toc"
	if !strings.Contains(string(out), want) {
		t.Fatalf("got %q, want substring %q", out, want)
	}
}

func TestFinalizeAnchorsHarborUntouched(t *testing.T) {
	tgt := target.Target{Registry: "harbor.example.com", Repository: "project/team/image"}

	src := "[toc](#toc)\n"

	out, err := FinalizeAnchors([]byte(src), provider.Harbor, tgt)
	if err != nil {
		t.Fatalf("FinalizeAnchors: %v", err)
	}

	if !strings.Contains(string(out), "(#toc)") {
		t.Fatalf("expected anchor untouched for harbor, got %q", out)
	}
}

func TestFinalizeAnchorsUnresolvedUntouched(t *testing.T) {
	tgt := target.Target{Registry: "custom.example.com", Repository: "project/image"}

	src := "[toc](#toc)\n"

	out, err := FinalizeAnchors([]byte(src), provider.Unknown, tgt)
	if err != nil {
		t.Fatalf("FinalizeAnchors: %v", err)
	}

	if string(out) != src {
		t.Fatalf("expected content byte-for-byte untouched, got %q", out)
	}
}

func TestFinalizeAnchorsNonAnchorLinksUntouched(t *testing.T) {
	tgt := target.Target{Registry: "quay.io", Repository: "group/image"}

	out, err := FinalizeAnchors([]byte("[x](https://example.com/a)\n"), provider.Quay, tgt)
	if err != nil {
		t.Fatalf("FinalizeAnchors: %v", err)
	}

	if !strings.Contains(string(out), "(https://example.com/a)") {
		t.Fatalf("expected absolute link untouched, got %q", out)
	}
}
