//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package guidance

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// frontmatter is the structured header parsed from each guidance doc. One
// field: everything else a doc needs is derived. The id is the filename and
// the name is the H1, so neither can disagree with the file it describes.
//
// yaml.Unmarshal ignores unknown keys, so a stale `id:` or `summary:` left in
// a doc is inert rather than fatal — which is what lets the corpus be replaced
// in one step rather than two.
type frontmatter struct {
	Tags []string `yaml:"tags"`
}

var fmMarker = []byte("---")

// parseFrontmatter splits a doc into its YAML front-matter and the remaining
// body. The file must open and close with a line that is exactly "---"
// (trailing whitespace and CR tolerated, so LF and CRLF docs both parse). A
// body horizontal rule or a malformed "----" line is not a delimiter, so it
// cannot truncate the header or leak into the metadata; otherwise it is a
// malformed guidance doc.
func parseFrontmatter(data []byte) (frontmatter, string, error) {
	var fm frontmatter
	nl := bytes.IndexByte(data, '\n')
	if nl < 0 || !bytes.Equal(bytes.TrimRight(data[:nl], " \t\r"), fmMarker) {
		return fm, "", fmt.Errorf("missing opening front-matter delimiter")
	}
	rest := data[nl+1:]

	var meta, body []byte
	found := false
	for off := 0; off < len(rest); {
		nl := bytes.IndexByte(rest[off:], '\n')
		line := rest[off:]
		if nl >= 0 {
			line = rest[off : off+nl]
		}
		if bytes.Equal(bytes.TrimRight(line, " \t\r"), fmMarker) {
			meta = rest[:off]
			if nl >= 0 {
				body = rest[off+nl+1:]
			}
			found = true
			break
		}
		if nl < 0 {
			break
		}
		off += nl + 1
	}
	if !found {
		return fm, "", fmt.Errorf("missing closing front-matter delimiter")
	}
	if err := yaml.Unmarshal(meta, &fm); err != nil {
		return fm, "", fmt.Errorf("invalid front-matter yaml: %w", err)
	}
	return fm, string(body), nil
}
