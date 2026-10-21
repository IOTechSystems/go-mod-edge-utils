//
// Copyright (C) 2026 IOTech Ltd
//

package metatool

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connect serves server over an in-memory transport and returns a client.
func connect(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "metatool-test-client", Version: "0"}, nil).
		Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func registered(t *testing.T, domains string) map[string]*mcp.Tool {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "metatool-test", Version: "0"}, nil)
	Register(server, domains)
	res, err := connect(t, server).ListTools(context.Background(), nil)
	require.NoError(t, err)
	out := map[string]*mcp.Tool{}
	for _, tl := range res.Tools {
		out[tl.Name] = tl
	}
	return out
}

func TestIsMetaTool(t *testing.T) {
	for _, name := range []string{SearchTools, CallReadTool, CallWriteTool} {
		assert.True(t, IsMetaTool(name), name)
	}
	assert.False(t, IsMetaTool("search_guidance"))
}

func TestRegister_ToolsAndAnnotations(t *testing.T) {
	tools := registered(t, "Covers X.")

	require.Len(t, tools, 3)
	for _, name := range []string{SearchTools, CallReadTool} {
		require.Contains(t, tools, name)
		a := tools[name].Annotations
		require.NotNil(t, a)
		assert.True(t, a.ReadOnlyHint, name)
		require.NotNil(t, a.DestructiveHint, name)
		assert.False(t, *a.DestructiveHint, name)
	}
	require.Contains(t, tools, CallWriteTool)
	a := tools[CallWriteTool].Annotations
	require.NotNil(t, a)
	assert.False(t, a.ReadOnlyHint)
	require.NotNil(t, a.DestructiveHint)
	assert.True(t, *a.DestructiveHint)

	assert.Contains(t, tools[SearchTools].Description, "Covers X.")
}

func TestRegister_EmptyDomains(t *testing.T) {
	desc := registered(t, "")[SearchTools].Description

	assert.NotContains(t, desc, "  ")
	assert.NotContains(t, desc, ". .")
	assert.Equal(t, strings.TrimSpace(desc), desc)
}

func TestRegister_NotInterceptedWithoutMiddleware(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "metatool-test", Version: "0"}, nil)
	Register(server, "")

	res, err := connect(t, server).CallTool(context.Background(), &mcp.CallToolParams{
		Name: SearchTools, Arguments: map[string]any{},
	})

	require.Error(t, err, "want a JSON-RPC error, got result %+v", res)
	assert.Contains(t, err.Error(), "handled by the meta-tool middleware")
}
