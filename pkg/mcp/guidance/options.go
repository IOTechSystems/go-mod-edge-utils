//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package guidance

import (
	"fmt"
	"regexp"
)

// Options carries the domain rules the engine enforces but does not itself
// define. A consumer supplies its own docs (via the fs.FS passed to New) and
// its own rules here, so the same engine serves different catalogs.
type Options struct {
	// Root is the directory within the filesystem to walk for docs. Empty
	// means DefaultRoot ("docs").
	Root string

	// Vocabulary is the closed set of tags a doc may carry. Empty means the
	// vocabulary is open: any tag is accepted. A closed set turns a tag typo —
	// which otherwise selects nothing, silently and permanently — into a
	// build-time failure.
	Vocabulary []string

	// MaxBodyBytes caps one document's body. Zero means DefaultMaxBodyBytes.
	MaxBodyBytes int

	// PointerTool is the tool name a document uses to cross-reference another
	// document, written as a call: <PointerTool>(id: "<doc-id>"). Empty
	// disables pointer-resolution checks. When set, every such call must parse
	// canonically and resolve to a known doc, or the build fails.
	PointerTool string

	// Scheme prefixes a doc's MCP resource URI, e.g. "guidance://".
	Scheme string

	// MIMEType is what docs are served as on the resource metadata and the
	// resources/read contents, e.g. "text/markdown".
	MIMEType string

	// pointerRe and pointerCall are compiled from PointerTool by compile().
	pointerRe   *regexp.Regexp
	pointerCall string
}

func (o Options) withDefaults() Options {
	if o.Root == "" {
		o.Root = DefaultRoot
	}
	if o.MaxBodyBytes == 0 {
		o.MaxBodyBytes = DefaultMaxBodyBytes
	}
	return o
}

// compile builds the pointer matchers from PointerTool. A pointer is written as
// a call so the model does not have to work out how to follow it, and so the
// corpus validator can check that it resolves.
//
//	<PointerTool>(id: "<doc-id>")
//
// \s* because a pointer near the right margin wraps, and a wrapped call is
// still the same call. pointerCall is the literal every pointer starts with:
// call sites are found by this literal and then required to parse as pointerRe,
// so a spelling the resolver cannot read fails the build instead of going
// unnoticed.
func (o *Options) compile() error {
	if o.PointerTool == "" {
		return nil
	}
	re, err := regexp.Compile(`^` + regexp.QuoteMeta(o.PointerTool) + `\(\s*id:\s*"([^"]+)"\s*\)`)
	if err != nil {
		return fmt.Errorf("guidance: invalid PointerTool %q: %w", o.PointerTool, err)
	}
	o.pointerRe = re
	o.pointerCall = o.PointerTool + "("
	return nil
}
