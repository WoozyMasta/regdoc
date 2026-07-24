// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package target parses and normalizes container image references into the
// registry/repository pair used for publishing a description.
package target

import (
	"fmt"
	"net"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
)

// Target is a normalized container repository reference.
// Tag and digest are discarded because they do not affect repository description publishing.
type Target struct {
	Original   string // Original is the user-supplied image reference.
	Registry   string // Registry is the normalized registry host and optional port.
	Repository string // Repository is the normalized repository path without tag or digest.
}

// Parse parses raw into a normalized Target, discarding any tag or digest.
func Parse(raw string) (Target, error) {
	ref, err := name.ParseReference(raw)
	if err != nil {
		return Target{}, fmt.Errorf("parse image reference %q: %w", raw, err)
	}

	repo := ref.Context()

	return Target{
		Original:   raw,
		Registry:   repo.RegistryStr(),
		Repository: repo.RepositoryStr(),
	}, nil
}

// ExplicitTag reports the tag literally present in raw, if any.
// It returns ok=false for a bare repository reference,
// an implied ":latest", or a digest reference -
// only a tag the user actually typed is a version-gate candidate.
func ExplicitTag(raw string) (tag string, ok bool) {
	t, err := name.NewTag(raw, name.WithDefaultTag(""))
	if err != nil {
		// Digest references (e.g. "...@sha256:...")
		// fail NewTag's own tag/repository split before NewRepository even runs;
		// treat that the same as "no explicit tag" rather than propagating an error.
		return "", false
	}

	if t.TagStr() == "" {
		return "", false
	}

	return t.TagStr(), true
}

// Hostname returns the registry hostname, lowercased, without port and without a trailing DNS root dot.
func (t Target) Hostname() string {
	host := t.Registry
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	host = strings.ToLower(host)
	host = strings.TrimSuffix(host, ".")

	return host
}
