//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package guidance

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func readGuidance(store *Store) mcp.ResourceHandler {
	return func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		id := strings.TrimPrefix(req.Params.URI, store.Scheme())
		d, err := store.Get(id)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", req.Params.URI, err)
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI: req.Params.URI, MIMEType: store.MIMEType(), Text: d.Body,
			}},
		}, nil
	}
}

// RegisterGuidance exposes every guidance doc as an MCP resource
// (<scheme><id>). Adding a resource enables the server's resources capability
// automatically. The URI scheme and MIME type come from the store's Options, so
// a search tool advertising store.URI(id) and this registrar cannot disagree.
func RegisterGuidance(server *mcp.Server, store *Store) {
	h := readGuidance(store)
	for _, d := range store.List(nil) {
		// No Description: front-matter is a single field, and the H1 that
		// becomes Name is the only summary a doc carries.
		server.AddResource(&mcp.Resource{
			URI: store.URI(d.ID), Name: d.Name, MIMEType: store.MIMEType(),
		}, h)
	}
}
