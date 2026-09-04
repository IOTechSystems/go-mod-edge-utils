//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package guidance

import (
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	src := "---\n" +
		"id: onboard-modbus-device\n" +
		"name: Onboard Modbus Device\n" +
		"summary: Step-by-step creation of a Modbus device\n" +
		"tags: [devices, modbus]\n" +
		"tools: [query_devices, manage_device]\n" +
		"---\n" +
		"# Body\n\nHello.\n"

	fm, body, err := parseFrontmatter([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// tags is the only field. The stale id/name/summary/tools keys above are
	// deliberately still in the fixture: yaml.Unmarshal ignores unknown keys, and
	// that is what lets a doc left over from the old contract be inert rather
	// than fatal.
	if len(fm.Tags) != 2 || fm.Tags[0] != "devices" {
		t.Fatalf("bad tags: %v", fm.Tags)
	}
	if !strings.HasPrefix(body, "# Body") {
		t.Fatalf("body not stripped: %q", body)
	}
}

func TestParseFrontmatterMissingDelimiter(t *testing.T) {
	if _, _, err := parseFrontmatter([]byte("no front matter here")); err == nil {
		t.Fatal("expected error for missing front-matter delimiter")
	}
}

func TestParseFrontmatterRejectsNonExactDelimiter(t *testing.T) {
	// A "----" line (or any line merely starting with "---") is not a valid
	// closing delimiter; only a line that is exactly "---" closes the header.
	src := "---\ntags: [devices]\n----\nbody\n"
	if _, _, err := parseFrontmatter([]byte(src)); err == nil {
		t.Fatal("expected error: '----' is not a valid closing delimiter")
	}
}

func TestParseFrontmatterAcceptsCRLF(t *testing.T) {
	// A CRLF-authored doc must parse: the opening delimiter tolerates \r just
	// like the closing one does.
	src := "---\r\ntags: [devices]\r\n---\r\nbody line\r\n"
	fm, body, err := parseFrontmatter([]byte(src))
	if err != nil {
		t.Fatalf("CRLF doc rejected: %v", err)
	}
	if len(fm.Tags) != 1 || fm.Tags[0] != "devices" {
		t.Fatalf("bad tags: %v", fm.Tags)
	}
	if !strings.Contains(body, "body line") {
		t.Fatalf("bad body: %q", body)
	}
}

func TestParseFrontmatterKeepsBodyHorizontalRules(t *testing.T) {
	// The first exact "---" line closes the header; later "---" rules stay in
	// the body verbatim.
	src := "---\ntags: [devices]\n---\nintro\n\n---\n\nmore\n"
	fm, body, err := parseFrontmatter([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm.Tags) != 1 {
		t.Fatalf("bad tags: %v", fm.Tags)
	}
	if !strings.Contains(body, "\n---\n") || !strings.Contains(body, "more") {
		t.Fatalf("body horizontal rule lost: %q", body)
	}
}
