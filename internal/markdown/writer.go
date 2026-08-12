// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package markdown

import (
	"bytes"
	"io"
	"unicode"
	"unicode/utf8"
)

// lineDelimiter separates rendered Markdown lines.
const lineDelimiter byte = '\n'

// linePrefix applies a prefix to a configurable line range.
type linePrefix struct {
	bytes     []byte // bytes contains the prefix text to emit.
	startLine int    // startLine is the first output line that receives the prefix.
	endLine   int    // endLine is the last output line that receives the prefix, or -1 forever.
}

// markdownWriter writes Markdown while managing prefixes and line flushing.
type markdownWriter struct {
	err                        error        // err stores the first write error.
	output                     io.Writer    // output is the final writer receiving rendered bytes.
	prefixes                   []linePrefix // prefixes contains active line prefixes.
	buffer                     bytes.Buffer // buffer stores incomplete output until a line is complete.
	lineBuffer                 bytes.Buffer // lineBuffer assembles a single output line before writing.
	line                       int          // line is the zero-based output line index.
	preserveTrailingWhitespace bool         // preserveTrailingWhitespace protects code block contents.
}

// newMarkdownWriter creates a Markdown writer around an output writer.
func newMarkdownWriter(output io.Writer) *markdownWriter {
	return &markdownWriter{output: output}
}

// writeLine writes bytes and flushes the current line.
func (w *markdownWriter) writeLine(line []byte) {
	w.writeBytes(line)
	w.flushLine()
}

// flushLine flushes the buffered line when it contains data.
func (w *markdownWriter) flushLine() {
	if w.buffer.Len() > 0 {
		w.endLine()
	}
}

// endLine terminates the current line.
func (w *markdownWriter) endLine() {
	w.writeBytes([]byte{lineDelimiter})
}

// pushPrefix adds a line prefix, optionally limited to a line range.
func (w *markdownWriter) pushPrefix(value []byte, lineRanges ...int) {
	prefix := linePrefix{bytes: value, endLine: -1}
	if len(lineRanges) > 0 {
		prefix.startLine = w.line + lineRanges[0]
		if len(lineRanges) > 1 {
			prefix.endLine = prefix.startLine + lineRanges[1]
		}
	}
	w.prefixes = append(w.prefixes, prefix)
}

// popPrefix removes the most recently added line prefix.
func (w *markdownWriter) popPrefix() {
	w.prefixes = w.prefixes[:len(w.prefixes)-1]
}

// writeBytes buffers bytes and writes completed lines with active prefixes.
func (w *markdownWriter) writeBytes(data []byte) int {
	if w.err != nil {
		return 0
	}

	n, _ := w.buffer.Write(data)
	for {
		lineEnd := bytes.IndexByte(w.buffer.Bytes(), lineDelimiter)
		if lineEnd < 0 {
			break
		}

		line := w.buffer.Next(lineEnd + 1)
		w.lineBuffer.Reset()
		for _, prefix := range w.prefixes {
			if prefix.startLine <= w.line && (prefix.endLine == -1 || w.line <= prefix.endLine) {
				w.lineBuffer.Write(prefix.bytes)
			}
		}
		w.lineBuffer.Write(line)

		if w.preserveTrailingWhitespace {
			w.lineBuffer.Truncate(w.lineBuffer.Len() - 1)
			if bytes.HasSuffix(w.lineBuffer.Bytes(), []byte{'\r'}) {
				w.lineBuffer.Truncate(w.lineBuffer.Len() - 1)
			}
		} else {
			trimmed := bytes.TrimRightFunc(w.lineBuffer.Bytes(), unicode.IsSpace)
			w.lineBuffer.Truncate(len(trimmed))
		}
		w.lineBuffer.WriteByte(lineDelimiter)

		if _, err := w.output.Write(w.lineBuffer.Bytes()); err != nil {
			w.err = err
			return 0
		}

		w.line++
	}

	return n
}

// writeRune writes a single rune through writeBytes.
func (w *markdownWriter) writeRune(value rune) (int, error) {
	var buffer [4]byte
	n := utf8.EncodeRune(buffer[:], value)
	return w.writeBytes(buffer[:n]), w.err
}

// errorState returns the first write error seen by the writer.
func (w *markdownWriter) errorState() error {
	return w.err
}

// flush flushes any buffered line and returns the write error state.
func (w *markdownWriter) flush() error {
	w.flushLine()
	return w.err
}
