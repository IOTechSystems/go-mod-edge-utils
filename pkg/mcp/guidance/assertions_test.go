//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package guidance_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/guidance"
)

const okDoc = "---\ntags: [devices]\n---\n# Title\n\nbody\n"

// Control. Without it every assertion below is satisfied by a New() that
// always fails.
func TestValidCorpusBuilds(t *testing.T) {
	mustBuild(t, map[string]string{"a.md": okDoc})
}

func TestTagsAreRequired(t *testing.T) {
	mustFail(t, map[string]string{"a.md": "---\ntags: []\n---\n# T\n\nbody\n"},
		"at least one tag")
}

// A typo produces a tag that selects nothing, silently, forever. A closed
// vocabulary turns that into a startup failure.
func TestTagsMustBeInVocabulary(t *testing.T) {
	mustFail(t, map[string]string{"a.md": "---\ntags: [devices, devcies]\n---\n# T\n\nbody\n"},
		`unknown tag "devcies"`)
}

// An empty Options.Vocabulary means the vocabulary is open: any tag is
// accepted, so a consumer that does not want a closed set is not forced into
// one. Tags are still required.
func TestOpenVocabularyAcceptsAnyTag(t *testing.T) {
	fsys := mapFS(map[string]string{
		"docs/a.md": "---\ntags: [anything, at-all]\n---\n# A\n\nbody\n",
	})
	_, err := guidance.New(fsys, guidance.Options{Scheme: "x://", MIMEType: "text/markdown"})
	require.NoError(t, err, "open vocabulary must accept any tag")
}

// The assertion that keeps the corpus small. Without it it grows back into a
// second copy of the product documentation.
func TestBodyByteBudget(t *testing.T) {
	big := "---\ntags: [devices]\n---\n# T\n\n" + strings.Repeat("x", guidance.DefaultMaxBodyBytes+1) + "\n"
	mustFail(t, map[string]string{"a.md": big}, "exceeds the body budget")
}

func TestH1IsRequired(t *testing.T) {
	mustFail(t, map[string]string{"a.md": "---\ntags: [devices]\n---\n\nbody with no heading\n"},
		"no H1")
}

func TestPointersMustResolve(t *testing.T) {
	mustFail(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\nSee `get_guidance(id: \"nope\")`.\n",
	}, `points at unknown document "nope"`)
}

// An empty Options.PointerTool disables pointer-resolution checks: a consumer
// whose docs do not cross-reference is not forced to configure one, and a
// pointer-shaped string is then just prose.
func TestNoPointerToolSkipsPointerChecks(t *testing.T) {
	fsys := mapFS(map[string]string{
		"docs/a.md": "---\ntags: [devices]\n---\n# A\n\nSee `get_guidance(id: \"nope\")`.\n",
	})
	_, err := guidance.New(fsys, guidance.Options{
		Vocabulary: []string{"devices"}, Scheme: "x://", MIMEType: "text/markdown",
	})
	require.NoError(t, err, "with no PointerTool a pointer-shaped string is inert")
}

// A pointer the resolver cannot read is worse than one that does not resolve:
// the model still follows it and startup says nothing. Only the canonical form
// is validated, so every other spelling must be refused rather than skipped.
func TestNonCanonicalPointerIsRejected(t *testing.T) {
	for _, body := range []string{
		"See `get_guidance(id: 'b')`.\n",
		"See `get_guidance({\"id\": \"b\"})`.\n",
		"See `get_guidance(\"b\")`.\n",
		"See `get_guidance(id=\"b\")`.\n",
	} {
		mustFail(t, map[string]string{
			"a.md": "---\ntags: [devices]\n---\n# A\n\n" + body,
			"b.md": okDoc,
		}, "must be written")
	}
}

// A pointer near the right margin really does break across lines. Wrapped is
// still canonical, and must resolve.
func TestWrappedPointerIsCanonical(t *testing.T) {
	mustBuild(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\nRead `get_guidance(id:\n\"b\")` for the rest.\n",
		"b.md": okDoc,
	})
}

// …and a wrapped pointer at an id that does not exist must still be caught: a
// detector that cannot cross a newline stops checking these at all.
func TestWrappedPointerToUnknownDocIsRejected(t *testing.T) {
	mustFail(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\nRead `get_guidance(id:\n\"no-such-doc\")`.\n",
	}, `points at unknown document "no-such-doc"`)
}

// A parenthetical after the id is not the canonical form, and must be refused
// rather than skipped for being unmatchable.
func TestPointerWithParensIsRejected(t *testing.T) {
	mustFail(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\nRead `get_guidance(id: \"b\" (deprecated))`.\n",
		"b.md": okDoc,
	}, "must be written")
}

// Every bad pointer must be reported, not just the first: each round-trip to
// fix one is a rebuild.
func TestAllBadPointersAreReported(t *testing.T) {
	mustFail(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\n" +
			"See `get_guidance(id: \"nope-one\")` and `get_guidance(id: \"nope-two\")`.\n",
	}, "nope-two")
	mustFail(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\n" +
			"See `get_guidance(id: \"nope-one\")` and `get_guidance(id: \"nope-two\")`.\n",
	}, "nope-one")
}

func TestResolvablePointerIsAccepted(t *testing.T) {
	mustBuild(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\nSee `get_guidance(id: \"b\")`.\n",
		"b.md": okDoc,
	})
}

// A fact with two homes has two things to keep in step, and nothing keeping
// them.
func TestNoDuplicatedLine(t *testing.T) {
	line := "The field is `updateDevices`, not `devices`."
	mustFail(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\n" + line + "\n",
		"b.md": "---\ntags: [devices]\n---\n# B\n\n" + line + "\n",
	}, "appears in both")
}

// Short and structural lines repeat legitimately. Without this the duplication
// check would forbid two documents from both using a bullet.
func TestShortLinesMayRepeat(t *testing.T) {
	mustBuild(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\n- one\n\n```json\n{}\n```\n",
		"b.md": "---\ntags: [devices]\n---\n# B\n\n- one\n\n```json\n{}\n```\n",
	})
}

// A pointer is one fact's second door, not its second home; a bare URL likewise.
// TestNoDuplicatedLine is the control that this exemption did not gut the rule.
func TestPointerAndURLLinesMayRepeat(t *testing.T) {
	pointer := "Only what differs from `get_guidance(id: \"b\")` — read that for the rest.\n"
	url := "https://docs.iotechsys.com/edge-central40/device-services/modbus/modbus-overview.html\n"
	mustBuild(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\n" + pointer + url,
		"c.md": "---\ntags: [devices]\n---\n# C\n\n" + pointer + url,
		"b.md": okDoc,
	})
}

// A wrapped pointer must be exempt on BOTH its lines, or the two rules disagree
// about wrapping: the continuation would read as ordinary prose and collide
// between two documents, and the failure would name a line fragment.
func TestWrappedPointerLinesMayRepeat(t *testing.T) {
	wrapped := "Only what differs from `get_guidance(id:\n\"b\")` — read that document for the rest of it.\n"
	mustBuild(t, map[string]string{
		"a.md": "---\ntags: [devices]\n---\n# A\n\n" + wrapped,
		"c.md": "---\ntags: [devices]\n---\n# C\n\n" + wrapped,
		"b.md": okDoc,
	})
}

// The id is the filename, so ids are unique only if the tree is flat — and the
// walk is recursive. Without this, two files at different depths share an id
// and the later silently wins.
func TestNestedDocumentIsRejected(t *testing.T) {
	mustFail(t, map[string]string{
		"a/update-device.md": "---\ntags: [devices]\n---\n# From A\n\nalpha\n",
		"b/update-device.md": "---\ntags: [devices]\n---\n# From B\n\nbeta\n",
	}, "must be flat")
}

// Even a uniquely-named nested file is rejected: the rule is flatness, not
// merely absence of collision, because a nested tree is what makes collision
// possible in the first place.
func TestNestedDocumentIsRejectedEvenWithoutCollision(t *testing.T) {
	mustFail(t, map[string]string{
		"devices/only-one.md": "---\ntags: [devices]\n---\n# Only\n\nbody\n",
	}, "must be flat")
}

// toolNameErr builds a one-document corpus and runs the tool-name assertion the
// controller runs at startup, returning its error.
func toolNameErr(t *testing.T, body string, known ...string) error {
	t.Helper()
	fsys := fstest.MapFS{
		"docs/a.md": &fstest.MapFile{Data: []byte("---\ntags: [devices]\n---\n# A\n\n" + body)},
	}
	s, err := guidance.New(fsys, testOpts())
	require.NoError(t, err)

	k := map[string]bool{}
	for _, n := range known {
		k[n] = true
	}
	return s.ValidateToolNames(k)
}

// A rename leaves stale names in both forms a document uses. Prose names a tool
// as a bare backticked identifier, so a check that only reads the call form
// passes while the prose points at a tool that no longer exists.
func TestBareToolNameMustBeRegistered(t *testing.T) {
	err := toolNameErr(t, "Create it with `manage_devcie`.\n", "manage_device")

	require.Error(t, err, "a bare backticked unknown tool must fail startup")
	assert.Contains(t, err.Error(), `names "manage_devcie"`)
}

// Control: the same form with a registered name is clean, so the check selects
// on the name and not on the backticks.
func TestBareToolNameThatExistsIsAccepted(t *testing.T) {
	assert.NoError(t, toolNameErr(t, "Create it with `manage_device`.\n", "manage_device"))
}

// The call form next to the bare form, so widening the pattern cannot drop it.
func TestCalledToolNameMustBeRegistered(t *testing.T) {
	err := toolNameErr(t, "Run `get_devcie(name: 1)`.\n", "get_device")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `names "get_devcie"`)
}
