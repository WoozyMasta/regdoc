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

	"oras.land/oras-go/v2/registry"
)

const (
	dockerRegistry = "docker.io"
	dockerIndex    = "index.docker.io"
	libraryPrefix  = "library/"
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
	ref, err := parseReference(raw)
	if err != nil {
		return Target{}, fmt.Errorf("parse image reference %q: %w", raw, err)
	}

	return Target{
		Original:   raw,
		Registry:   ref.Registry,
		Repository: ref.Repository,
	}, nil
}

// ExplicitTag reports the tag literally present in raw, if any.
// It returns ok=false for a bare repository reference,
// an implied ":latest", or a digest reference -
// only a tag the user actually typed is a version-gate candidate.
func ExplicitTag(raw string) (tag string, ok bool) {
	if strings.Contains(raw, "@") {
		return "", false
	}

	ref, err := parseReference(raw)
	if err != nil || ref.Reference == "" {
		return "", false
	}

	return ref.Reference, true
}

// parseReference applies Docker's familiar-name defaults
// before validating the fully qualified reference with ORAS.
func parseReference(raw string) (registry.Reference, error) {
	qualified := raw
	first, _, hasSlash := strings.Cut(raw, "/")

	if !hasSlash {
		qualified = dockerRegistry + "/" + libraryPrefix + raw
	} else if !isRegistryComponent(first) {
		qualified = dockerRegistry + "/" + raw
	}

	ref, err := registry.ParseReference(qualified)
	if err != nil {
		return registry.Reference{}, err
	}

	if isDockerHubRegistry(ref.Registry) {
		ref.Registry = dockerIndex
		if !strings.Contains(ref.Repository, "/") {
			ref.Repository = libraryPrefix + ref.Repository
		}
	}

	return ref, nil
}

// isDockerHubRegistry reports whether registry
// is a Docker Hub endpoint or familiar-name alias.
func isDockerHubRegistry(registry string) bool {
	return registry == dockerRegistry || registry == dockerIndex
}

// isRegistryComponent reports whether the first path component identifies a
// registry according to Docker's familiar-reference convention.
func isRegistryComponent(component string) bool {
	return component == "localhost" ||
		strings.ContainsAny(component, ".:") ||
		strings.HasPrefix(component, "[")
}

// Hostname returns the registry hostname, lowercased,
// without port and without a trailing DNS root dot.
func (t Target) Hostname() string {
	host := t.Registry
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	host = strings.ToLower(host)
	host = strings.TrimSuffix(host, ".")

	return host
}
