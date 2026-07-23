// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"bytes"
	"unicode/utf8"
)

// Cut limits content to limit bytes.
// It prefers a Markdown or HTML heading no deeper than headingLevel,
// then a blank line, before making a UTF-8-safe cut.
func Cut(content []byte, limit, headingLevel int) []byte {
	if limit <= 0 {
		return nil
	}

	if len(content) <= limit {
		return content
	}

	limit = utf8Boundary(content, limit)
	if boundary := headingBoundary(content, limit, headingLevel); boundary > 0 {
		return content[:boundary]
	}

	if boundary := bytes.LastIndex(content[:limit], []byte("\n\n")); boundary > 0 {
		return content[:boundary]
	}

	return content[:limit]
}

// LimitRunes limits content to limit Unicode code points without splitting a rune.
func LimitRunes(content []byte, limit int) []byte {
	if limit <= 0 || utf8.RuneCount(content) <= limit {
		return content
	}

	count := 0
	for i := range content {
		if !utf8.RuneStart(content[i]) {
			continue
		}

		if count == limit {
			return content[:i]
		}
		count++
	}

	return content
}

// headingBoundary finds the last eligible Markdown or HTML heading before limit.
func headingBoundary(content []byte, limit, headingLevel int) int {
	if headingLevel < 1 {
		return 0
	}

	if headingLevel > 6 {
		headingLevel = 6
	}

	boundary := 0
	for lineStart := 0; lineStart < limit; {
		lineEnd := bytes.IndexByte(content[lineStart:limit], '\n')
		if lineEnd < 0 {
			lineEnd = limit
		} else {
			lineEnd += lineStart
		}

		markdownLevel := markdownHeadingLevel(content[lineStart:lineEnd])
		htmlLevel := htmlHeadingLevel(content[lineStart:lineEnd])
		if markdownLevel > 0 && markdownLevel <= headingLevel ||
			htmlLevel > 0 && htmlLevel <= headingLevel {
			boundary = lineStart
		}

		if lineEnd == limit {
			break
		}
		lineStart = lineEnd + 1
	}

	return boundary
}

// markdownHeadingLevel returns zero for a non-heading line.
func markdownHeadingLevel(line []byte) int {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}

	if level == 0 || level > 6 || level == len(line) || line[level] != ' ' {
		return 0
	}

	return level
}

// htmlHeadingLevel returns zero for a line that does not start an HTML heading.
func htmlHeadingLevel(line []byte) int {
	if len(line) < 4 || line[0] != '<' || line[1] != 'h' || line[2] < '1' || line[2] > '6' {
		return 0
	}

	if line[3] != '>' && line[3] != ' ' {
		return 0
	}

	return int(line[2] - '0')
}

// utf8Boundary moves a byte limit left until it starts a complete UTF-8 rune.
func utf8Boundary(content []byte, limit int) int {
	if limit >= len(content) {
		return len(content)
	}

	for limit > 0 && !utf8.RuneStart(content[limit]) {
		limit--
	}

	return limit
}
