// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/regdoc

package document

import (
	"bytes"
	"unicode/utf8"
)

type codeFence struct {
	marker byte
	length int
}

// Cut limits content to limit bytes.
// It prefers a Markdown or HTML heading no deeper than headingLevel,
// then a blank line or complete line outside fenced code,
// before making a UTF-8-safe cut when no line boundary exists.
func Cut(content []byte, limit, headingLevel int) []byte {
	if limit <= 0 {
		return nil
	}

	if len(content) <= limit {
		return content
	}

	limit = utf8Boundary(content, limit)
	if boundary := markdownCutBoundary(content, limit, headingLevel); boundary >= 0 {
		return content[:boundary]
	}

	return content[:limit]
}

// markdownCutBoundary returns the preferred safe boundary outside fenced code.
func markdownCutBoundary(content []byte, limit, headingLevel int) int {
	heading, blank := -1, -1
	fenceStart := -1
	var open codeFence

	lineStart := 0
	for line := range bytes.SplitSeq(content, []byte{'\n'}) {
		lineEnd := lineStart + len(line)
		lineComplete := lineEnd <= limit
		candidate, rest, isFence := parseFence(line)
		switch {
		case open.length > 0:
			if lineComplete && isFence && candidate.marker == open.marker &&
				candidate.length >= open.length && len(bytes.Trim(rest, " \t\r")) == 0 {
				open = codeFence{}
			}
		case isFence && (candidate.marker != '`' || !bytes.ContainsRune(rest, '`')):
			open = candidate
			fenceStart = lineStart
		case lineComplete:
			markdownLevel := markdownHeadingLevel(line)
			htmlLevel := htmlHeadingLevel(line)
			if markdownLevel > 0 && markdownLevel <= headingLevel ||
				htmlLevel > 0 && htmlLevel <= headingLevel {
				heading = lineStart
			}
			if len(bytes.TrimSpace(line)) == 0 && lineStart > 0 {
				blank = lineStart - 1
			}
		}
		if lineEnd >= limit || lineEnd == len(content) {
			break
		}
		lineStart = lineEnd + 1
	}

	if open.length > 0 {
		return len(bytes.TrimRight(content[:fenceStart], " \t\r\n"))
	}
	if heading > 0 {
		return heading
	}
	if blank > 0 {
		return blank
	}
	if lineEnd := bytes.LastIndexByte(content[:limit], '\n'); lineEnd > 0 {
		if content[lineEnd-1] == '\r' {
			lineEnd--
		}
		return lineEnd
	}

	return -1
}

// parseFence parses a fence marker after up to three leading spaces.
func parseFence(line []byte) (codeFence, []byte, bool) {
	indent := 0
	for indent < 3 && indent < len(line) && line[indent] == ' ' {
		indent++
	}
	if indent == len(line) || line[indent] != '`' && line[indent] != '~' {
		return codeFence{}, nil, false
	}

	marker := line[indent]
	length := 0
	for indent+length < len(line) && line[indent+length] == marker {
		length++
	}
	if length < 3 {
		return codeFence{}, nil, false
	}

	return codeFence{marker: marker, length: length}, line[indent+length:], true
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
