//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package guidance_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/guidance"
)

// testOpts is the domain configuration the engine tests run under: a closed
// vocabulary the fixtures draw from, and the pointer/scheme/MIME a consumer
// would inject. Kept small — the fixtures only need these tags.
func testOpts() guidance.Options {
	return guidance.Options{
		Vocabulary:  []string{"devices", "modbus", "create"},
		PointerTool: "get_guidance",
		Scheme:      "guidance://",
		MIMEType:    "text/markdown",
	}
}

func mapFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return fsys
}

// build constructs a store from docs/<name> fixtures under testOpts.
func build(t *testing.T, files map[string]string) (*guidance.Store, error) {
	t.Helper()
	prefixed := map[string]string{}
	for name, body := range files {
		prefixed["docs/"+name] = body
	}
	return guidance.New(mapFS(prefixed), testOpts())
}

func mustFail(t *testing.T, files map[string]string, wantSubstring string) {
	t.Helper()
	_, err := build(t, files)
	require.Error(t, err, "this corpus must not build")
	assert.Contains(t, err.Error(), wantSubstring)
}

func mustBuild(t *testing.T, files map[string]string) {
	t.Helper()
	_, err := build(t, files)
	require.NoError(t, err)
}
