// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package target

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		name           string
		raw            string
		wantRegistry   string
		wantRepository string
		wantErr        bool
	}{
		{
			name:           "docker hub full",
			raw:            "docker.io/user/image",
			wantRegistry:   "index.docker.io",
			wantRepository: "user/image",
		},
		{
			name:           "docker hub shorthand adds library namespace",
			raw:            "user/image",
			wantRegistry:   "index.docker.io",
			wantRepository: "user/image",
		},
		{
			name:           "docker hub bare official image",
			raw:            "ubuntu",
			wantRegistry:   "index.docker.io",
			wantRepository: "library/ubuntu",
		},
		{
			name:           "custom registry with port and tag",
			raw:            "registry.example.com:5000/project/image:tag",
			wantRegistry:   "registry.example.com:5000",
			wantRepository: "project/image",
		},
		{
			name:           "nested repository path",
			raw:            "registry.example.com/project/team/image",
			wantRegistry:   "registry.example.com",
			wantRepository: "project/team/image",
		},
		{
			name:           "digest reference",
			raw:            "quay.io/group/image@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantRegistry:   "quay.io",
			wantRepository: "group/image",
		},
		{
			name:    "invalid reference",
			raw:     "INVALID//not a ref::",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.raw)
				}

				return
			}

			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.raw, err)
			}

			if got.Registry != tc.wantRegistry {
				t.Errorf("Registry = %q, want %q", got.Registry, tc.wantRegistry)
			}

			if got.Repository != tc.wantRepository {
				t.Errorf("Repository = %q, want %q", got.Repository, tc.wantRepository)
			}

			if got.Original != tc.raw {
				t.Errorf("Original = %q, want %q", got.Original, tc.raw)
			}
		})
	}
}

func TestExplicitTag(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantTag string
		wantOK  bool
	}{
		{
			name: "bare repository, no tag",
			raw:  "registry.example.com/org/image",
		},
		{
			name:    "explicit tag",
			raw:     "registry.example.com/org/image:3.1.1",
			wantTag: "3.1.1",
			wantOK:  true,
		},
		{
			name:    "explicit latest still counts as explicit",
			raw:     "registry.example.com/org/image:latest",
			wantTag: "latest",
			wantOK:  true,
		},
		{
			name: "digest reference has no tag",
			raw:  "quay.io/group/image@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "registry with port, no tag",
			raw:  "registry.example.com:5000/org/image",
		},
		{
			name:    "registry with port and tag",
			raw:     "registry.example.com:5000/org/image:3.1.1",
			wantTag: "3.1.1",
			wantOK:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotTag, gotOK := ExplicitTag(tc.raw)
			if gotTag != tc.wantTag || gotOK != tc.wantOK {
				t.Errorf("ExplicitTag(%q) = (%q, %v), want (%q, %v)", tc.raw, gotTag, gotOK, tc.wantTag, tc.wantOK)
			}
		})
	}
}

func TestHostname(t *testing.T) {
	cases := []struct {
		registry string
		want     string
	}{
		{"registry.example.com:5000", "registry.example.com"},
		{"Quay.IO", "quay.io"},
		{"quay.io.", "quay.io"},
		{"index.docker.io", "index.docker.io"},
	}

	for _, tc := range cases {
		got := Target{Registry: tc.registry}.Hostname()
		if got != tc.want {
			t.Errorf("Hostname(%q) = %q, want %q", tc.registry, got, tc.want)
		}
	}
}
