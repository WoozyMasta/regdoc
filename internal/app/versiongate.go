// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"context"
	"fmt"

	"github.com/woozymasta/rats"

	"github.com/woozymasta/regdoc/internal/provider"
	"github.com/woozymasta/regdoc/internal/target"
)

// tagOrderGate reports whether publishing must be skipped
// as a deliberate no-op because IMAGE's explicit tag is older
// than the highest existing stable tag already published in tgt's repository,
// both compared under cfg.VersionFormat.
//
// It is a no-op (skip=false, err=nil) whenever IMAGE carries no explicit tag,
// the tag does not match the configured version format,
// or the repository has no comparable existing tag.
func tagOrderGate(
	ctx context.Context, cfg Config, lister provider.TagLister,
	tgt target.Target, reporter Reporter,
) (skip bool, err error) {
	rawTag, ok := target.ExplicitTag(cfg.Positional.Image)
	if !ok {
		return false, nil
	}

	config, err := buildVersionConfig(cfg)
	if err != nil {
		return false, err
	}

	selector, err := rats.Compile(config)
	if err != nil {
		return false, fmt.Errorf("compile --version-format %s: %w", cfg.VersionFormat, err)
	}

	incoming := selector.Select([]string{rawTag})
	if len(incoming.Items) == 0 {
		reporter.Debugf("tag %q does not match --version-format %s, skipping tag-order check", rawTag, cfg.VersionFormat)
		return false, nil
	}

	existing, err := lister.ListTags(ctx, tgt)
	if err != nil {
		return false, fmt.Errorf("list existing tags for %s: %w", tgt.Repository, err)
	}

	baseline := selector.Select(existing)
	if len(baseline.Items) == 0 {
		return false, nil
	}

	comparison, compatible := incoming.Items[0].Version.Compare(baseline.Items[0].Version)
	if compatible && comparison < 0 {
		reporter.Infof("skip publish: tag %s is older than already-published %s", rawTag, baseline.Labels[0])
		return true, nil
	}

	return false, nil
}

// buildVersionConfig builds a single-route RATS configuration
// that selects the single highest comparable value under cfg.VersionFormat.
// The same config gates both the incoming tag and the existing-tag baseline,
// so a format's own notion of "comparable" (e.g. SemVer's prerelease exclusion)
// applies uniformly to both.
func buildVersionConfig(cfg Config) (rats.Config, error) {
	route, err := buildVersionRoute(cfg)
	if err != nil {
		return rats.Config{}, err
	}

	return rats.Config{
		Routes: []rats.Route{route},
		Query:  rats.Query{Order: rats.OrderDescending, Limit: 1},
	}, nil
}

// buildVersionRoute selects the RATS route for cfg.VersionFormat.
// SemVer (the default) excludes prerelease and build metadata from comparison,
// so only stable releases gate publishing.
// Other formats use RATS' own strict-syntax defaults, matching its CLI.
func buildVersionRoute(cfg Config) (rats.Route, error) {
	switch cfg.VersionFormat {
	case "", "semver":
		return rats.SemVer(rats.SemVerModeRelaxed, rats.SemVerOptions{
			Prerelease: rats.MetadataFilter{Mode: rats.MetadataAbsent},
			Build:      rats.MetadataFilter{Mode: rats.MetadataAbsent},
		}), nil

	case "calver":
		var format rats.CalVerFormat
		if err := format.UnmarshalText([]byte(cfg.CalVerFormat)); err != nil {
			return nil, fmt.Errorf("invalid --calver-format %q: %w", cfg.CalVerFormat, err)
		}

		return rats.CalVerPresetRoute(format)

	case "numeric":
		return rats.Numeric(rats.NumericOptions{}), nil

	case "lexical":
		return rats.Lexical(rats.LexicalModeFolded, rats.LexicalOptions{}), nil

	case "pep440":
		return rats.PEP440(rats.PEP440Options{RequirePEP440Syntax: true}), nil

	case "debian":
		return rats.Debian(rats.DebianOptions{RequireDebianSyntax: true}), nil

	case "rpm":
		return rats.RPM(rats.RPMOptions{RequireRPMSyntax: true}), nil

	default:
		return nil, fmt.Errorf("unknown --version-format %q", cfg.VersionFormat)
	}
}
