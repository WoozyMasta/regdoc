// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"context"
	"fmt"

	"github.com/woozymasta/rats"
	"github.com/woozymasta/semver"

	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

// tagOrderGate reports whether publishing must be skipped as a deliberate no-op
// because IMAGE's explicit tag is older than the highest existing stable tag
// already published in tgt's repository.
// It is a no-op (skip=false, err=nil) whenever IMAGE carries no explicit tag,
// the tag isn't a valid stable release,
// or the repository has no comparable existing stable tag.
func tagOrderGate(
	ctx context.Context, cfg Config, lister provider.TagLister,
	tgt target.Target, reporter Reporter,
) (skip bool, err error) {
	rawTag, ok := target.ExplicitTag(cfg.Positional.Image)
	if !ok {
		return false, nil
	}

	incoming, valid := semver.Parse(rawTag)
	if !valid || !incoming.IsRelease() {
		reporter.Debugf("tag %q is not a stable release version, skipping tag-order check", rawTag)
		return false, nil
	}

	existing, err := lister.ListTags(ctx, tgt)
	if err != nil {
		return false, fmt.Errorf("list existing tags for %s: %w", tgt.Repository, err)
	}

	latest := rats.Latest(existing)
	if len(latest) == 0 {
		return false, nil
	}

	baseline, valid := semver.Parse(latest[0])
	if !valid {
		return false, nil // defensive: should not happen given rats' own filtering.
	}

	if incoming.Compare(baseline) < 0 {
		reporter.Infof("skip publish: tag %s is older than already-published %s", rawTag, latest[0])
		return true, nil
	}

	return false, nil
}
