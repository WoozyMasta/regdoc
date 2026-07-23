// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package app

import (
	"fmt"
	"io"
)

// Reporter is the minimal diagnostics sink used across the app package.
// stdout is reserved for the resulting Markdown; Reporter always writes to stderr.
type Reporter interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

// writerReporter emits diagnostics to one writer.
type writerReporter struct {
	w     io.Writer
	debug bool
	quiet bool
}

// NewReporter builds a Reporter writing to w. debug enables Debugf output;
// quiet suppresses Infof output. Warnf is never suppressed.
func NewReporter(w io.Writer, debug, quiet bool) Reporter {
	return &writerReporter{w: w, debug: debug, quiet: quiet}
}

// Debugf implements Reporter.
func (r *writerReporter) Debugf(format string, args ...any) {
	if !r.debug {
		return
	}

	fmt.Fprintf(r.w, "debug: "+format+"\n", args...) //nolint:errcheck // best-effort diagnostics.
}

// Infof implements Reporter.
func (r *writerReporter) Infof(format string, args ...any) {
	if r.quiet {
		return
	}

	fmt.Fprintf(r.w, format+"\n", args...) //nolint:errcheck // best-effort diagnostics.
}

// Warnf implements Reporter.
func (r *writerReporter) Warnf(format string, args ...any) {
	fmt.Fprintf(r.w, "warning: "+format+"\n", args...) //nolint:errcheck // best-effort diagnostics.
}
