//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package guidance is a reusable engine for a small catalog of embedded
// markdown "guidance" documents served over MCP: it parses a flat directory of
// front-matter'd .md files, validates the corpus at build time, and exposes it
// as MCP resources.
//
// The engine defines the mechanism only. Everything domain-specific — the
// closed tag vocabulary, the body budget, the cross-reference tool name, and
// the resource URI scheme and MIME type — is injected through Options, so each
// consumer supplies its own docs and its own rules. See Options.
package guidance

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// idRe limits a doc id to a slug: alphanumerics with - _ . ~ as internal
// separators. Those four separators are all RFC 3986 unreserved, so the id goes
// into the resource URI verbatim without escaping or changing how the URI
// parses. The id is the only field concatenated into the advertised URI (see
// Store.URI), so an id with a space, %, #, or ? would either make that URI
// invalid or push part of it into a query/fragment — a resource that fails to
// read back rather than one that fails at startup.
var idRe = regexp.MustCompile(`^[A-Za-z0-9]+(?:[-_.~][A-Za-z0-9]+)*$`)

// DefaultRoot is the directory within the filesystem walked for docs when
// Options.Root is empty.
const DefaultRoot = "docs"

// DefaultMaxBodyBytes caps one document's body when Options.MaxBodyBytes is 0.
//
// Guidance is a sequence of tool calls, not a manual: a consumer's domain lives
// in its own documentation and is linked, never restated. Without a hard cap
// the corpus grows back into a second copy of that documentation. A document
// that will not fit is a granularity problem, not a budget problem.
const DefaultMaxBodyBytes = 2048

// Doc is one guidance document. Four fields, three of them derived: the id is
// the filename, the name is the H1, the body is what is left.
type Doc struct {
	ID, Name string
	Tags     []string
	Body     string
}

// Store holds the in-memory guidance catalog built once at startup.
type Store struct {
	byID  map[string]*Doc
	order []*Doc
	opts  Options
}

// New walks Options.Root, validates every file, and builds the catalog. Any
// violation is a fatal content bug and returns an error, so the service does
// not start with a corpus that would mislead a model.
//
// Tool names are checked separately, by ValidateToolNames — this package
// cannot import a consumer's tool registry.
func New(fsys fs.FS, opts Options) (*Store, error) {
	opts = opts.withDefaults()
	if err := opts.compile(); err != nil {
		return nil, err
	}
	var entries []string
	err := fs.WalkDir(fsys, opts.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			entries = append(entries, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(entries) // deterministic scan order regardless of walk/OS
	s := &Store{byID: map[string]*Doc{}, opts: opts}
	prefix := opts.Root + "/"
	for _, path := range entries {
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		fm, body, err := parseFrontmatter(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		// The id is the filename, so two files at different depths would share
		// one id: the later would overwrite byID while both stayed in order,
		// listing the id twice and registering <scheme><id> twice. Rejecting
		// nesting is what actually makes "two files cannot collide" true —
		// deriving the id from the base name does not, on its own.
		if strings.Contains(strings.TrimPrefix(path, prefix), "/") {
			return nil, fmt.Errorf("%s: %s must be flat; the id is the filename, "+
				"so a nested file could collide with one at another depth", path, opts.Root)
		}
		id := strings.TrimSuffix(filepath.Base(path), ".md")
		if !idRe.MatchString(id) {
			return nil, fmt.Errorf("%s: id %q is not URI-safe; the id is the "+
				"filename and goes verbatim into the resource URI, so it must be a "+
				"slug matching %s", path, id, idRe)
		}
		name, rest, err := splitH1(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		d := &Doc{ID: id, Name: name, Tags: fm.Tags, Body: rest}
		if err := s.validate(d); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		s.byID[id] = d
		s.order = append(s.order, d)
	}
	if err := s.validateCorpus(); err != nil {
		return nil, err
	}
	return s, nil
}

// splitH1 takes the first "# " line as the document's name and returns the
// rest as the body. The name is the MCP Resource.Name, so a doc without one
// would register a nameless resource.
func splitH1(body string) (name, rest string, err error) {
	lines := strings.Split(body, "\n")
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "# ") {
			break
		}
		return strings.TrimSpace(strings.TrimPrefix(t, "# ")),
			strings.TrimSpace(strings.Join(lines[i+1:], "\n")), nil
	}
	return "", "", fmt.Errorf("no H1: the first non-blank line must be \"# <title>\"")
}

// clone returns a deep copy whose mutable Tags slice is independent of the
// store's. The store is built once and thereafter only read, so handing callers
// copies keeps that immutability enforced (not by convention) even if a caller
// mutates what it receives.
func (d *Doc) clone() Doc {
	c := *d
	if d.Tags != nil {
		c.Tags = append([]string(nil), d.Tags...)
	}
	return c
}

// List returns every doc carrying ALL of the given tags, in filename order.
// No tags means the whole catalog.
//
// AND, not OR: a union over ["devices","modbus"] returns every device doc,
// which is not a filter. There is no ranking — with a catalog this small the
// model picks by id, and a scorer would be a second thing to keep honest.
func (s *Store) List(tags []string) []Doc {
	out := make([]Doc, 0, len(s.order))
	for _, d := range s.order {
		if !hasAllTags(d, tags) {
			continue
		}
		out = append(out, d.clone())
	}
	return out
}

// hasAllTags is case-insensitive: a mis-cased tag would otherwise select
// nothing and read as "there is no guidance for this" rather than as a
// rejected argument.
func hasAllTags(d *Doc, want []string) bool {
	for _, w := range want {
		if !slices.ContainsFunc(d.Tags, func(have string) bool {
			return strings.EqualFold(have, w)
		}) {
			return false
		}
	}
	return true
}

// Get returns an independent copy of one doc by id, or an error.
func (s *Store) Get(id string) (*Doc, error) {
	d, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("unknown guidance id %q", id)
	}
	c := d.clone()
	return &c, nil
}

// Tags returns the sorted, distinct set of tags across all docs — the
// vocabulary a search tool advertises as its tag-filter enum, so the model
// only ever filters by tags that actually exist.
func (s *Store) Tags() []string {
	set := map[string]struct{}{}
	for _, d := range s.order {
		for _, t := range d.Tags {
			set[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Scheme returns the configured resource URI scheme (e.g. "guidance://").
func (s *Store) Scheme() string { return s.opts.Scheme }

// MIMEType returns the configured MIME type docs are served with.
func (s *Store) MIMEType() string { return s.opts.MIMEType }

// URI returns the MCP resource URI for a doc id: Scheme + id. The registrar and
// any search tool both build the advertised URI here, so the two cannot drift.
func (s *Store) URI(id string) string { return s.opts.Scheme + id }
