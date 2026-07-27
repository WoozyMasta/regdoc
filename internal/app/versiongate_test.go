// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/woozymasta/regdoc/internal/target"
)

// stubTagLister returns a fixed tag list or error, without any HTTP involved.
type stubTagLister struct {
	tags []string
	err  error
}

func (s stubTagLister) ListTags(_ context.Context, _ target.Target) ([]string, error) {
	return s.tags, s.err
}

func testGateConfig(image string) Config {
	cfg := testConfig("")
	cfg.Positional.Image = image

	return cfg
}

func TestTagOrderGateSkipsOlderTag(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:1.0.0")
	lister := stubTagLister{tags: []string{"1.0.0", "2.0.0"}}

	var stderr bytes.Buffer
	reporter := NewReporter(&stderr, false, false)

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, reporter)
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if !skip {
		t.Fatal("expected skip=true for a tag older than the existing latest")
	}
}

func TestTagOrderGatePublishesNewerTag(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:2.1.0")
	lister := stubTagLister{tags: []string{"1.0.0", "2.0.0"}}

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if skip {
		t.Fatal("expected skip=false for a tag newer than the existing latest")
	}
}

func TestTagOrderGatePublishesEqualTag(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:2.0.0")
	lister := stubTagLister{tags: []string{"1.0.0", "2.0.0"}}

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if skip {
		t.Fatal("expected skip=false for a re-run of the same tag")
	}
}

func TestTagOrderGateNoExplicitTagAlwaysPublishes(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image")
	lister := stubTagLister{tags: []string{"99.0.0"}}

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if skip {
		t.Fatal("expected skip=false when IMAGE carries no explicit tag")
	}
}

func TestTagOrderGateNonSemverTagAlwaysPublishes(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:latest")
	lister := stubTagLister{tags: []string{"99.0.0"}}

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if skip {
		t.Fatal("expected skip=false for a non-semver explicit tag")
	}
}

func TestTagOrderGatePrereleaseIncomingTagAlwaysPublishes(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:3.0.0-rc.1")
	lister := stubTagLister{tags: []string{"99.0.0"}}

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if skip {
		t.Fatal("expected skip=false for a prerelease explicit tag")
	}
}

func TestTagOrderGateEmptyExistingTagsAlwaysPublishes(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:1.0.0")
	lister := stubTagLister{tags: nil}

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if skip {
		t.Fatal("expected skip=false when the repository has no existing tags")
	}
}

func TestTagOrderGateOnlyPrereleaseExistingTagsAlwaysPublishes(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:1.0.0")
	lister := stubTagLister{tags: []string{"2.0.0-rc.1", "2.0.0-rc.2"}}

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if skip {
		t.Fatal("expected skip=false when no existing tag is a stable release")
	}
}

func TestTagOrderGateListTagsFailureIsError(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:1.0.0")
	lister := stubTagLister{err: errors.New("boom")}

	_, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err == nil {
		t.Fatal("expected error when listing existing tags fails")
	}
}

func TestTagOrderGateCalVerSkipsOlderTag(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:2026-01-01")
	cfg.VersionFormat = "calver"
	cfg.CalVerFormat = "ymd-dash"
	lister := stubTagLister{tags: []string{"2026-06-01", "2026-01-15"}}

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if !skip {
		t.Fatal("expected skip=true for a CalVer date older than the existing latest")
	}
}

func TestTagOrderGateCalVerPublishesNewerTag(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:2026-08-01")
	cfg.VersionFormat = "calver"
	cfg.CalVerFormat = "ymd-dash"
	lister := stubTagLister{tags: []string{"2026-06-01", "2026-01-15"}}

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if skip {
		t.Fatal("expected skip=false for a CalVer date newer than the existing latest")
	}
}

func TestTagOrderGateCalVerIgnoresSemverLookingExistingTags(t *testing.T) {
	// Tags that don't match the configured --calver-format layout are simply not comparable,
	// the same way a non-SemVer tag is ignored under the default format.
	cfg := testGateConfig("registry.example/group/image:2026-01-01")
	cfg.VersionFormat = "calver"
	cfg.CalVerFormat = "ymd-dash"
	lister := stubTagLister{tags: []string{"1.0.0", "2.0.0", "latest"}}

	skip, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err != nil {
		t.Fatalf("tagOrderGate: %v", err)
	}

	if skip {
		t.Fatal("expected skip=false when no existing tag matches the CalVer layout")
	}
}

func TestTagOrderGateInvalidCalVerFormatIsError(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:2026-01-01")
	cfg.VersionFormat = "calver"
	cfg.CalVerFormat = "bogus"
	lister := stubTagLister{tags: []string{"2026-01-01"}}

	_, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err == nil {
		t.Fatal("expected error for an unrecognized --calver-format value")
	}
}

func TestTagOrderGateUnknownVersionFormatIsError(t *testing.T) {
	cfg := testGateConfig("registry.example/group/image:1.0.0")
	cfg.VersionFormat = "not-a-real-format"
	lister := stubTagLister{tags: []string{"1.0.0"}}

	_, err := tagOrderGate(context.Background(), cfg, lister, target.Target{}, NewReporter(&bytes.Buffer{}, false, false))
	if err == nil {
		t.Fatal("expected error for an unknown --version-format value")
	}
}
