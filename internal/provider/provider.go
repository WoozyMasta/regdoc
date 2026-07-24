// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

// Package provider defines the provider-facing domain types and the
// Publisher contract implemented by the dockerhub, quay and harbor clients.
package provider

import (
	"context"

	"github.com/woozymasta/regdoc/internal/target"
)

// Type identifies a supported container registry provider.
type Type string

// Supported provider types.
// Unknown means the provider could not be determined.
const (
	Unknown   Type = ""
	DockerHub Type = "dockerhub"
	Quay      Type = "quay"
	Harbor    Type = "harbor"
)

// Document is the final, rendered payload ready to publish as a repository description.
type Document struct {
	Content          []byte   // Content is the rendered repository description.
	ShortDescription string   // ShortDescription is provider-specific summary text.
	Sources          []string // Sources lists input paths included in Content.
}

// Publisher publishes a Document as the description of Target.
type Publisher interface {
	Publish(ctx context.Context, tgt target.Target, doc Document) error
}

// TagLister lists tags already published in tgt's repository.
// Publisher implementations may optionally support it;
// callers type-assert rather than requiring it universally.
type TagLister interface {
	ListTags(ctx context.Context, tgt target.Target) ([]string, error)
}

// String returns the provider type as displayed to the user
// (CLI value, error messages, debug output).
func (t Type) String() string {
	if t == Unknown {
		return "unknown"
	}

	return string(t)
}
