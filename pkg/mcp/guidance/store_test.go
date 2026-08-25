//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package guidance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The id is the filename. Nothing declares it, so nothing can disagree with it.
// Uniqueness does NOT follow from that on its own — the walk is recursive, so
// two files at different depths would share a base name; New rejects nesting,
// and TestNestedDocumentIsRejected is what makes the id unique.
func TestIDComesFromFilename(t *testing.T) {
	s, err := build(t, map[string]string{
		"onboard-device.md": "---\ntags: [devices, create]\n---\n# Onboard a device\nbody\n",
	})
	require.NoError(t, err)

	d, err := s.Get("onboard-device")
	require.NoError(t, err)
	assert.Equal(t, "onboard-device", d.ID)
}

// The id is concatenated verbatim into the resource URI (Store.URI), so a
// filename with a character that is unsafe there — a space or %, which make the
// URI invalid, or a # or ?, which push part of it into a fragment/query — must
// fail at startup rather than advertise a URI that cannot be read back.
func TestNonURISafeIDIsRejected(t *testing.T) {
	for _, name := range []string{
		"my doc.md",   // space
		"50%off.md",   // bare percent
		"intro#v2.md", // fragment delimiter
		"faq?.md",     // query delimiter
	} {
		t.Run(name, func(t *testing.T) {
			mustFail(t, map[string]string{
				name: "---\ntags: [devices]\n---\n# T\n\nbody\n",
			}, "not URI-safe")
		})
	}
}

// The RFC 3986 unreserved separators are safe in a URI verbatim, so an id using
// them builds.
func TestURISafeIDBuilds(t *testing.T) {
	for _, name := range []string{
		"onboard-device.md",
		"modbus_devices.md",
		"v1.2.md",
		"a~b.md",
		"MixedCase.md",
	} {
		t.Run(name, func(t *testing.T) {
			mustBuild(t, map[string]string{
				name: "---\ntags: [devices]\n---\n# T\n\nbody\n",
			})
		})
	}
}

// The H1 is the MCP Resource.Name. Without it the resource has no name.
func TestNameComesFromH1(t *testing.T) {
	s, err := build(t, map[string]string{
		"x.md": "---\ntags: [devices]\n---\n# Onboard a Modbus device\n\nbody\n",
	})
	require.NoError(t, err)

	d, err := s.Get("x")
	require.NoError(t, err)
	assert.Equal(t, "Onboard a Modbus device", d.Name)
}

func TestBodyExcludesTheH1(t *testing.T) {
	s, err := build(t, map[string]string{
		"x.md": "---\ntags: [devices]\n---\n# Title\n\nthe body\n",
	})
	require.NoError(t, err)

	d, err := s.Get("x")
	require.NoError(t, err)
	assert.NotContains(t, d.Body, "# Title")
	assert.Contains(t, d.Body, "the body")
}

func TestGetRejectsUnknownID(t *testing.T) {
	s, err := build(t, map[string]string{"x.md": "---\ntags: [devices]\n---\n# T\n\nbody\n"})
	require.NoError(t, err)

	_, err = s.Get("nope")
	assert.Error(t, err)
}

// Tags filter with AND. The store is the only thing that decides this, so the
// case lives here as well as at the tool boundary.
func TestListFiltersByAllTags(t *testing.T) {
	s, err := build(t, map[string]string{
		"a.md": "---\ntags: [devices, modbus]\n---\n# A\n\nbody\n",
		"b.md": "---\ntags: [devices]\n---\n# B\n\nbody\n",
	})
	require.NoError(t, err)

	got := s.List([]string{"devices", "modbus"})
	require.Len(t, got, 1)
	assert.Equal(t, "a", got[0].ID)
	assert.Len(t, s.List(nil), 2)
}

// The schema enum makes a mis-cased tag unlikely from a well-behaved client,
// but without this the failure is an empty catalog, which reads as "there is no
// guidance for this" rather than as a rejected argument.
func TestListTagsAreCaseInsensitive(t *testing.T) {
	s, err := build(t, map[string]string{
		"a.md": "---\ntags: [devices, modbus]\n---\n# A\n\nbody\n",
	})
	require.NoError(t, err)

	require.Len(t, s.List([]string{"Devices"}), 1)
	require.Len(t, s.List([]string{"DEVICES", "Modbus"}), 1)
	assert.Empty(t, s.List([]string{"devices", "create"}), "AND still excludes a tag the doc lacks")
}

// The store is built once and thereafter only read, so it hands out copies —
// a caller that mutates what it receives cannot corrupt the catalog.
func TestStoreReturnsIsolatedCopies(t *testing.T) {
	s, err := build(t, map[string]string{
		"a.md": "---\ntags: [devices, modbus]\n---\n# A\n\nbody\n",
	})
	require.NoError(t, err)

	d, err := s.Get("a")
	require.NoError(t, err)
	d.Tags[0] = "MUTATED"

	d2, err := s.Get("a")
	require.NoError(t, err)
	assert.NotEqual(t, "MUTATED", d2.Tags[0], "store Tags corrupted via returned Doc")

	list := s.List(nil)
	list[0].Tags[0] = "MUTATED2"
	d3, err := s.Get("a")
	require.NoError(t, err)
	assert.NotEqual(t, "MUTATED2", d3.Tags[0], "store Tags corrupted via List")
}

func TestStoreTags(t *testing.T) {
	s, err := build(t, map[string]string{
		"a.md": "---\ntags: [devices, modbus]\n---\n# A\n\nbody\n",
		"b.md": "---\ntags: [devices]\n---\n# B\n\nbody\n",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"devices", "modbus"}, s.Tags(), "sorted and distinct")
}

// URI is where the registrar and any search tool both build the advertised
// resource URI, so they cannot drift from the configured scheme.
func TestStoreURI(t *testing.T) {
	s, err := build(t, map[string]string{"a.md": "---\ntags: [devices]\n---\n# A\n\nbody\n"})
	require.NoError(t, err)
	assert.Equal(t, "guidance://a", s.URI("a"))
	assert.Equal(t, "guidance://", s.Scheme())
	assert.Equal(t, "text/markdown", s.MIMEType())
}
