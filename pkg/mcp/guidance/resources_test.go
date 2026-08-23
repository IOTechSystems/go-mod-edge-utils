//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package guidance

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestReadGuidanceResource(t *testing.T) {
	store := newTestStore(t)
	h := readGuidance(store)
	res, err := h(context.Background(), &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: "guidance://onboard-modbus-device"}})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(res.Contents) != 1 || res.Contents[0].Text == "" {
		t.Fatalf("bad contents: %+v", res)
	}
	if res.Contents[0].MIMEType != "text/markdown" {
		t.Fatalf("wrong mime: %q", res.Contents[0].MIMEType)
	}
	if _, err := h(context.Background(), &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: "guidance://nope"}}); err == nil {
		t.Fatal("expected error for unknown uri")
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	doc := "---\ntags: [devices, modbus]\n---\n# Onboard a Modbus device\n\nUse this for modbus.\n"
	fsys := fstest.MapFS{"docs/onboard-modbus-device.md": &fstest.MapFile{Data: []byte(doc)}}
	s, err := New(fsys, Options{
		Vocabulary: []string{"devices", "modbus"},
		Scheme:     "guidance://",
		MIMEType:   "text/markdown",
	})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return s
}
