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

// tagGateServer builds a mock Quay server dispatching tag-listing
// and description-publish requests, tracking whether publish was invoked.
func tagGateServer(t *testing.T, existingTags []string) (srv *httptest.Server, published *bool) {
	t.Helper()

	published = new(bool)

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/tag/"):
			w.Header().Set("Content-Type", "application/json")

			names := make([]string, len(existingTags))
			for i, tag := range existingTags {
				names[i] = `{"name":"` + tag + `"}`
			}

			_, _ = w.Write([]byte(`{"tags":[` + strings.Join(names, ",") + `],"page":1,"has_additional":false}`))
		default:
			*published = true
			w.WriteHeader(http.StatusOK)
		}
	}))

	return srv, published
}

func TestRunSkipsPublishForOlderExplicitTag(t *testing.T) {
	srv, published := tagGateServer(t, []string{"1.0.0", "2.0.0"})
	defer srv.Close()

	u, _ := url.Parse(srv.URL)

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "quay"
	cfg.PlainHTTP = true
	cfg.Token = "quay-token"
	cfg.Positional.Image = u.Host + "/group/image:1.5.0"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if *published {
		t.Fatal("expected publish to be skipped for a tag older than the existing latest")
	}
}

func TestRunPublishesForNewerExplicitTag(t *testing.T) {
	srv, published := tagGateServer(t, []string{"1.0.0", "2.0.0"})
	defer srv.Close()

	u, _ := url.Parse(srv.URL)

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "quay"
	cfg.PlainHTTP = true
	cfg.Token = "quay-token"
	cfg.Positional.Image = u.Host + "/group/image:2.1.0"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !*published {
		t.Fatal("expected publish for a tag newer than the existing latest")
	}
}

func TestRunNoExplicitTagAlwaysPublishes(t *testing.T) {
	srv, published := tagGateServer(t, []string{"99.0.0"})
	defer srv.Close()

	u, _ := url.Parse(srv.URL)

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "quay"
	cfg.PlainHTTP = true
	cfg.Token = "quay-token"
	cfg.Positional.Image = u.Host + "/group/image"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !*published {
		t.Fatal("expected publish when IMAGE carries no explicit tag, regardless of existing tags")
	}
}

func TestRunSkipTagCheckBypassesGate(t *testing.T) {
	srv, published := tagGateServer(t, []string{"9.0.0"})
	defer srv.Close()

	u, _ := url.Parse(srv.URL)

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "quay"
	cfg.PlainHTTP = true
	cfg.Token = "quay-token"
	cfg.SkipTagCheck = true
	cfg.Positional.Image = u.Host + "/group/image:1.0.0"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !*published {
		t.Fatal("expected --skip-tag-check to bypass the gate and publish an older tag anyway")
	}
}

func TestRunSkipsMarkdownProcessingForOlderExplicitTag(t *testing.T) {
	srv, published := tagGateServer(t, []string{"1.0.0", "2.0.0"})
	defer srv.Close()

	u, _ := url.Parse(srv.URL)

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "quay"
	cfg.PlainHTTP = true
	cfg.Token = "quay-token"
	// An explicit license path that does not exist makes document.BuildHeader fail;
	// if the tag-order gate runs before document processing (as it must),
	// Run never reaches BuildHeader and this never surfaces.
	cfg.License = "does-not-exist.txt"
	cfg.Positional.Image = u.Host + "/group/image:1.5.0"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v (markdown/header processing must be skipped before it can fail)", err)
	}

	if *published {
		t.Fatal("expected publish to be skipped for a tag older than the existing latest")
	}
}

func TestRunReleaseVersionFlagWinsOverImageTag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Output = "-"
	cfg.ReleaseVersion = "2.0.0"
	cfg.Positional.Image = "custom.invalid.example/project/image:1.0.0"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := stdout.String()

	if !strings.Contains(out, "2.0.0") {
		t.Fatalf("expected --release-version to appear, got %q", out)
	}

	if strings.Contains(out, "1.0.0") {
		t.Fatalf("expected IMAGE's tag not to appear when --release-version is set, got %q", out)
	}
}

func TestRunReleaseVersionFallsBackToImageTag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Output = "-"
	cfg.Positional.Image = "custom.invalid.example/project/image:1.5.0"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if out := stdout.String(); !strings.Contains(out, "1.5.0") {
		t.Fatalf("expected IMAGE's tag to be used as the release version, got %q", out)
	}
}

func TestRunNoReleaseVersionOrTagOmitsReleaseLine(t *testing.T) {
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

	if out := stdout.String(); strings.Contains(out, "Release:") {
		t.Fatalf("expected no Release line without --release-version or an IMAGE tag, got %q", out)
	}
}

func TestRunReleaseVersionLinksToDiscoveredTagPage(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example/group/project")
	t.Setenv("CI_COMMIT_SHA", "0123456789abcdef0123456789abcdef01234567")

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Hello\n")

	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Output = "-"
	cfg.Positional.Image = "custom.invalid.example/project/image:1.2.3"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := "Release: [1.2.3](https://gitlab.example/group/project/-/tags/1.2.3)"
	if out := stdout.String(); !strings.Contains(out, want) {
		t.Fatalf("expected %q in output, got %q", want, out)
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

func TestRunRewritesLinksAndImagesWithExplicitBaseURLs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "[docs](guide.md)\n\n![logo](logo.png)\n")

	cfg := testConfig(dir)
	cfg.Provider = "auto"
	cfg.Output = "-"
	cfg.LinkBaseURL = "https://git.example/project/-/blob/main/"
	cfg.ImageBaseURL = "https://git.example/project/-/raw/main/"
	cfg.Positional.Image = "custom.invalid.example/project/image"

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), cfg, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := stdout.String()

	if !strings.Contains(out, "https://git.example/project/-/blob/main/guide.md") {
		t.Fatalf("expected link rewritten under LinkBaseURL, got %q", out)
	}

	if !strings.Contains(out, "https://git.example/project/-/raw/main/logo.png") {
		t.Fatalf("expected image rewritten under ImageBaseURL, got %q", out)
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
