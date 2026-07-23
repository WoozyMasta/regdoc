// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testConfig provides common defaults for application tests.
func testConfig(root string) Config {
	return Config{
		TargetOptions:  TargetOptions{Root: root},
		RuntimeOptions: RuntimeOptions{Timeout: 5 * time.Second},
	}
}

// writeFile creates a test document.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.WriteFile(
		filepath.Join(dir, name),
		[]byte(content),
		0o644,
	); err != nil { //nolint:gosec
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestRunEndToEndPublishesHTMLToQuay(t *testing.T) {
	var gotBody map[string]string

	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "quay"
	cfg.PlainHTTP = true
	cfg.Token = "quay-token"
	cfg.Fallback = FallbackCut
	cfg.Format = "html"
	cfg.UserAgent = "regdoc/test"
	cfg.Positional.Image = u.Host + "/group/image"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if gotAuth != "Bearer quay-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}

	if !strings.Contains(gotBody["description"], "<h1>Hello</h1>") {
		t.Fatalf("unexpected published body: %+v", gotBody)
	}
}

func TestRunOutputDashWritesStdoutNoNetwork(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Output = "-"
	cfg.Positional.Image = "custom.invalid.example/project/image"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(stdout.String(), "# Hello") {
		t.Fatalf("expected merged content on stdout, got %q", stdout.String())
	}
}

func TestRunOutputHTMLWritesRenderedDocument(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n\n| Name | Value |\n| --- | --- |\n| one | two |\n")

	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Output = "-"
	cfg.Format = "html"
	cfg.Positional.Image = "custom.invalid.example/project/image"

	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, want := range []string{"<h1>Hello</h1>", "<table>", "<td>one</td>"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}
func TestRunOutputEmbedsLocalImage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "![Logo](logo.png)\n")
	if err := os.WriteFile(
		filepath.Join(dir, "logo.png"),
		[]byte{0x89, 'P', 'N', 'G'},
		0o644,
	); err != nil {
		t.Fatalf("write image: %v", err)
	}

	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Output = "-"
	cfg.Format = "html"
	cfg.EmbedImages = true
	cfg.Positional.Image = "custom.invalid.example/project/image"

	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(stdout.String(), "<img src=\"data:image/png;base64,iVBORw==\"") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunOutputFileSkipsPublishing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	out := filepath.Join(dir, "merged.md")

	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Output = out
	cfg.Positional.Image = "quay.io/group/image"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected nothing on stdout when --output is set, got %q", stdout.String())
	}

	got, err := os.ReadFile(out) //nolint:gosec
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if !strings.Contains(string(got), "# Hello") {
		t.Fatalf("unexpected output file content: %q", got)
	}
}

func TestRunNoDocumentsIsNoop(t *testing.T) {
	dir := t.TempDir()

	cfg := testConfig(dir)
	cfg.Positional.Image = "quay.io/group/image"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunOptionalNoCredentialsIsNoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Optional = true
	cfg.Positional.Image = "quay.io/group/image"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunNoCredentialsWithoutOptionalIsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Positional.Image = "quay.io/group/image"

	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), cfg, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when no credentials are available and --optional is not set")
	}

	if !strings.Contains(err.Error(), "Quay OAuth token") {
		t.Fatalf("expected a Quay credentials error, got: %v", err)
	}
}

func TestRunInvalidImageIsConfigError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Positional.Image = "INVALID::not a ref"

	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), cfg, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid image reference")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
}

func TestRunOutputSkipsProviderDetectionAndPublishing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	out := filepath.Join(dir, "merged.md")
	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Output = out
	cfg.Positional.Image = "custom.invalid.example/project/image"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}

	content, err := os.ReadFile(out) //nolint:gosec
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if !strings.Contains(string(content), "# Hello") {
		t.Fatalf("output = %q", content)
	}
}
