//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package guidance

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// toolNameRe matches both forms a document names a tool in: as a call, and as a
// bare backticked identifier. Checking only the call form leaves the prose
// unguarded, so a rename with one call site updated would pass startup while
// the prose points at a tool that no longer exists.
//
// The underscore is what separates a tool name from ordinary prose in both
// forms: JSON fields in this domain are written camelCase, so a backticked
// snake_case identifier is a tool name.
var toolNameRe = regexp.MustCompile("\\b([a-z][a-z0-9]*(?:_[a-z0-9]+)+)\\(|`([a-z][a-z0-9]*(?:_[a-z0-9]+)+)`")

// ValidateToolNames rejects a document naming a tool that does not exist.
//
// Not part of New: this package cannot import a consumer's tool registry (the
// registry typically imports this package). A consumer that has both — usually
// its controller — calls this immediately after New, so a renamed tool still
// fails startup.
//
// known must be every registered tool, not only the upstream-mapped ones —
// documents point at each other with the pointer tool, which is local.
func (s *Store) ValidateToolNames(known map[string]bool) error {
	var problems []string
	for _, d := range s.order {
		for _, m := range toolNameRe.FindAllStringSubmatch(d.Body, -1) {
			name := m[1] // the call form; empty when the bare form matched
			if name == "" {
				name = m[2]
			}
			if !known[name] {
				problems = append(problems, fmt.Sprintf(
					"%s names %q, which is not a registered tool", d.ID, name))
			}
		}
	}
	return joined(problems)
}

// validate checks one document in isolation.
func (s *Store) validate(d *Doc) error {
	if len(d.Tags) == 0 {
		return fmt.Errorf("needs at least one tag")
	}
	if len(s.opts.Vocabulary) > 0 {
		for _, t := range d.Tags {
			if !slices.Contains(s.opts.Vocabulary, t) {
				return fmt.Errorf("unknown tag %q; the vocabulary is %v", t, s.opts.Vocabulary)
			}
		}
	}
	if n := len(d.Body); n > s.opts.MaxBodyBytes {
		return fmt.Errorf("body is %d bytes, which exceeds the body budget of %d", n, s.opts.MaxBodyBytes)
	}
	return nil
}

// validateCorpus checks the properties that only exist across documents.
func (s *Store) validateCorpus() error {
	if s.opts.pointerCall != "" {
		known := make(map[string]bool, len(s.order))
		for _, d := range s.order {
			known[d.ID] = true
		}
		var problems []string
		for _, d := range s.order {
			for off := 0; ; {
				i := strings.Index(d.Body[off:], s.opts.pointerCall)
				if i < 0 {
					break
				}
				at := off + i
				off = at + len(s.opts.pointerCall)

				m := s.opts.pointerRe.FindStringSubmatch(d.Body[at:])
				if m == nil {
					problems = append(problems, fmt.Sprintf(
						"%s: %s… must be written %sid: \"<doc-id>\")",
						d.ID, firstLine(d.Body[at:], 60), s.opts.pointerCall))
					continue
				}
				if !known[m[1]] {
					problems = append(problems, fmt.Sprintf(
						"%s points at unknown document %q", d.ID, m[1]))
				}
			}
		}
		if err := joined(problems); err != nil {
			return err
		}
	}
	return s.duplicateLines()
}

// duplicateLines rejects a line of substance that appears in two documents.
//
// A fact with two homes has nothing keeping the two in step. Shared facts
// belong in a backbone document that the others point at.
//
// Skipped: anything short or structural, which repeats legitimately; and any
// line carrying a pointer or a bare URL, which are a second door to one fact
// rather than a second copy of it. That exemption takes whole lines, so a
// duplicated fact with a pointer appended is not caught.
func (s *Store) duplicateLines() error {
	pointerCall := s.opts.pointerCall
	seen := map[string]string{} // line -> first doc id
	var dupes []string
	for _, d := range s.order {
		openPointer := false // a wrapped pointer's continuation is exempt too
		for _, raw := range strings.Split(d.Body, "\n") {
			ln := strings.TrimSpace(raw)
			wasOpen := openPointer
			if pointerCall != "" {
				if i := strings.LastIndex(ln, pointerCall); i >= 0 {
					openPointer = !strings.Contains(ln[i:], ")")
				} else if openPointer {
					openPointer = !strings.Contains(ln, ")")
				}
			}
			switch {
			case wasOpen,
				len(ln) < 40,
				strings.HasPrefix(ln, "#"), strings.HasPrefix(ln, "```"),
				strings.HasPrefix(ln, "|"), strings.HasPrefix(ln, "http"),
				pointerCall != "" && strings.Contains(ln, pointerCall):
				continue
			}
			if first, ok := seen[ln]; ok && first != d.ID {
				dupes = append(dupes, fmt.Sprintf("%q appears in both %s and %s", ln, first, d.ID))
				continue
			}
			seen[ln] = d.ID
		}
	}
	if len(dupes) > 0 {
		sort.Strings(dupes)
		return fmt.Errorf("duplicated content: %s", strings.Join(dupes, "; "))
	}
	return nil
}

// firstLine returns at most n bytes of s up to the first newline, with backticks
// stripped, for quoting a document's text back in an error.
func firstLine(s string, n int) string {
	if len(s) > n {
		s = s[:n]
	}
	return strings.ReplaceAll(strings.SplitN(s, "\n", 2)[0], "`", "")
}

// joined reports every problem at once, so one bad document does not hide the
// rest behind a rebuild.
func joined(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s", strings.Join(problems, "; "))
}
