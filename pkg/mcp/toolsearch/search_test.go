//
// Copyright (C) 2026 IOTech Ltd
//

package toolsearch

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readKind(string) string { return "read" }

func names(res Result) []string {
	out := make([]string, len(res.Tools))
	for i, e := range res.Tools {
		out[i] = e.Name
	}
	return out
}

func TestSummary(t *testing.T) {
	assert.Equal(t, "Lists devices.", summary("Lists devices. Use filters."))
	long := strings.Repeat("a", 200)
	assert.Equal(t, long[:160], summary(long))

	// Characters, not bytes: a 2-byte "é" is one of the 160.
	accents := strings.Repeat("é", 200)
	assert.Equal(t, strings.Repeat("é", 160), summary(accents))
}

func TestTokenize_PluralAndSplit(t *testing.T) {
	assert.Equal(t, []string{"query", "command", "for", "reading"}, tokenize("Query_Commands for READINGS"))
}

func TestSearch_Deterministic(t *testing.T) {
	catalogue := []*mcp.Tool{
		{Name: "query_devices", Description: "Lists devices."},
		{Name: "get_device", Description: "Gets one device."},
		{Name: "manage_device", Description: "Adds, updates or deletes devices."},
		{Name: "query_readings", Description: "Lists readings of a device."},
	}
	assert.Equal(t, Search(catalogue, "device", readKind), Search(catalogue, "device", readKind))
}

func TestSearch_ExactName(t *testing.T) {
	schema := map[string]any{"type": "object"}
	catalogue := []*mcp.Tool{
		{Name: "get_device", Description: "Get one device.", InputSchema: schema},
		{Name: "get_device_profile", Description: "Get one device profile."},
		{Name: "query_devices", Description: "List devices."},
	}
	res := Search(catalogue, "get_device", readKind)

	assert.Equal(t, 1, res.Total)
	require.Len(t, res.Tools, 1)
	assert.Equal(t, "get_device", res.Tools[0].Name)
	assert.Equal(t, "Get one device.", res.Tools[0].Description)
	assert.Equal(t, schema, res.Tools[0].InputSchema)
}

func TestSearch_Ranking(t *testing.T) {
	catalogue := []*mcp.Tool{
		{Name: "get_device", Description: "Get one device."},
		{Name: "query_devices", Description: "List devices."},
		{Name: "get_device_resource", Description: "Get a device resource of a profile."},
		{Name: "get_device_profile", Description: "Get one device profile."},
		{Name: "query_device_profiles", Description: "List device profiles."},
		{Name: "manage_device_profile", Description: "Add, update or delete device profiles."},
		{Name: "query_readings", Description: "List readings of a device."},
		{Name: "ping_service", Description: "Ping a service."},
	}
	got := names(Search(catalogue, "device profile", readKind))

	require.GreaterOrEqual(t, len(got), 3)
	assert.ElementsMatch(t, []string{"get_device_profile", "manage_device_profile", "query_device_profiles"}, got[:3])
	assert.NotContains(t, got, "ping_service")
}

func TestSearch_CapAndNote(t *testing.T) {
	var catalogue []*mcp.Tool
	for i := 12; i >= 1; i-- {
		catalogue = append(catalogue, &mcp.Tool{Name: fmt.Sprintf("tool_%02d", i), InputSchema: map[string]any{}})
	}
	res := Search(catalogue, "", readKind)

	assert.Equal(t, 12, res.Total)
	assert.Equal(t, []string{"tool_01", "tool_02", "tool_03", "tool_04", "tool_05"}, names(res))
	assert.Equal(t, "Showing 5 of 12. Narrow the query.", res.Note)
	for _, e := range res.Tools {
		assert.Nil(t, e.InputSchema)
	}
}

func TestSearch_KindAndPropertyNames(t *testing.T) {
	kind := func(name string) string {
		if name == "manage_x" {
			return "write"
		}
		return "read"
	}
	catalogue := []*mcp.Tool{
		{Name: "manage_x", Description: "Change x."},
		{Name: "typed_schema", Description: "One.", InputSchema: &jsonschema.Schema{
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{"deviceName": {Type: "string"}},
		}},
		{Name: "map_schema", Description: "Two.", InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"deviceName": map[string]any{"type": "string"}},
		}},
	}

	res := Search(catalogue, "manage_x", kind)
	require.Len(t, res.Tools, 1)
	assert.Equal(t, "write", res.Tools[0].Kind)

	assert.ElementsMatch(t, []string{"map_schema", "typed_schema"}, names(Search(catalogue, "devicename", kind)))
}
