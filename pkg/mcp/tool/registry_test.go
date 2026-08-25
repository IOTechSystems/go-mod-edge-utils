//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package tool

import (
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testContainer stands in for a service's dependency container type.
type testContainer struct{}

// noopAdd is a stand-in for a real Add; Register only checks it is non-nil.
func noopAdd(*mcp.Server, testContainer) {}

// registerPanic registers tl into a fresh registry and returns the panic
// value, or nil.
func registerPanic(t *testing.T, tl Tool[testContainer]) (recovered any) {
	t.Helper()
	defer func() { recovered = recover() }()
	NewRegistry[testContainer]().Register(tl)
	return nil
}

// A tool that declares neither a route universe nor Local is invisible to every
// user, because an empty universe is fail-closed. Nothing else in the system
// reports that, so registration refuses it.
func TestRegister_PanicsWithNoRoutesAndNotLocal(t *testing.T) {
	got := registerPanic(t, Tool[testContainer]{Name: "zz_forgot_to_declare", Behaviour: ReadOnly, Add: noopAdd})

	require.NotNil(t, got, "a tool with no universe and no local marking must not register")
	assert.Contains(t, got, "declares no visibility routes and is not marked local")
}

// First control. Without it, a Register that panicked unconditionally would
// satisfy the assertion above.
func TestRegister_AcceptsDeclaredRoutes(t *testing.T) {
	got := registerPanic(t, Tool[testContainer]{
		Name:             "zz_declares_routes",
		Behaviour:        ReadOnly,
		ServiceKey:       "core-metadata",
		VisibilityRoutes: []Route{ServiceRoute("core-metadata", "/api/v3/device/all", http.MethodGet)},
		Add:              noopAdd,
	})

	assert.Nil(t, got, "a tool declaring its routes must register")
}

// Second control, for the other branch of the same condition.
func TestRegister_AcceptsExplicitLocal(t *testing.T) {
	got := registerPanic(t, Tool[testContainer]{Name: "zz_local", Local: true, Behaviour: ReadOnly, Add: noopAdd})

	assert.Nil(t, got, "a tool explicitly marked local must register")
}

// Local and a route universe together mean one of the two is wrong, and which is
// not guessable — a local tool with routes would be authorized against routes it
// never calls.
func TestRegister_PanicsOnLocalWithRoutes(t *testing.T) {
	got := registerPanic(t, Tool[testContainer]{
		Name:             "zz_local_with_routes",
		Local:            true,
		Behaviour:        ReadOnly,
		VisibilityRoutes: []Route{ServiceRoute("core-metadata", "/api/v3/device/all", http.MethodGet)},
		Add:              noopAdd,
	})

	require.NotNil(t, got)
	assert.Contains(t, got, "marked local but declares upstream routes")
}

// A local tool uses no upstream service, so giving it a service key would make it
// wait on something it never calls.
func TestRegister_PanicsOnLocalWithServiceKey(t *testing.T) {
	got := registerPanic(t, Tool[testContainer]{
		Name:       "zz_local_with_key",
		Local:      true,
		Behaviour:  ReadOnly,
		ServiceKey: "core-metadata",
		Add:        noopAdd,
	})

	require.NotNil(t, got)
	assert.Contains(t, got, "marked local but declares service key")
}

// A mapped tool with no service key skips the up-check and loads even when its
// upstream service is down.
func TestRegister_PanicsOnMappedWithoutServiceKey(t *testing.T) {
	got := registerPanic(t, Tool[testContainer]{
		Name:             "zz_mapped_no_key",
		Behaviour:        ReadOnly,
		VisibilityRoutes: []Route{ServiceRoute("core-metadata", "/api/v3/device/all", http.MethodGet)},
		Add:              noopAdd,
	})

	require.NotNil(t, got)
	assert.Contains(t, got, "declares no service key and is not marked local")
}

func TestRegister_PanicsOnDuplicateName(t *testing.T) {
	r := NewRegistry[testContainer]()
	r.Register(Tool[testContainer]{Name: "zz_dup", Local: true, Behaviour: ReadOnly, Add: noopAdd})

	var got any
	func() {
		defer func() { got = recover() }()
		r.Register(Tool[testContainer]{Name: "zz_dup", Local: true, Behaviour: ReadOnly, Add: noopAdd})
	}()

	require.NotNil(t, got)
	assert.Contains(t, got, "duplicate registration")
}

func TestRegister_PanicsOnMissingAdd(t *testing.T) {
	got := registerPanic(t, Tool[testContainer]{Name: "zz_no_add", Local: true, Behaviour: ReadOnly})

	require.NotNil(t, got)
	assert.Contains(t, got, "no Add function")
}

func TestRegister_PanicsOnEmptyName(t *testing.T) {
	got := registerPanic(t, Tool[testContainer]{Local: true, Behaviour: ReadOnly, Add: noopAdd})

	require.NotNil(t, got)
	assert.Contains(t, got, "empty name")
}

// A tool that declares no behaviour inherits the protocol default — go-sdk
// documents DestructiveHint as "Default: true" — so a forgotten declaration
// advertises a read-only tool as destructive, and clients gate approval on
// exactly that. The zero value is not a behaviour for exactly this reason.
func TestRegister_PanicsWithNoBehaviour(t *testing.T) {
	got := registerPanic(t, Tool[testContainer]{Name: "zz_no_hint", Local: true, Add: noopAdd})

	require.NotNil(t, got, "a tool declaring no behaviour must not register")
	assert.Contains(t, got, "must declare a Behaviour")
}

// An integer outside the set reaches AnnotationsFor as "neither read-only nor
// destructive", which is Additive — a wrong hint rather than a loud one.
func TestRegister_PanicsOnUnknownBehaviour(t *testing.T) {
	got := registerPanic(t, Tool[testContainer]{
		Name: "zz_bad_behaviour", Local: true, Behaviour: Behaviour(99), Add: noopAdd,
	})

	require.NotNil(t, got)
	assert.Contains(t, got, "Behaviour(99)")
}

// Control for the two assertions above: all three declared behaviours register,
// so neither can be satisfied by a Register that panics unconditionally.
func TestRegister_AcceptsEveryDeclaredBehaviour(t *testing.T) {
	for _, b := range []Behaviour{ReadOnly, Additive, Destructive} {
		assert.Nilf(t, registerPanic(t, Tool[testContainer]{
			Name: "zz_" + b.String(), Local: true, Behaviour: b, Add: noopAdd,
		}), "a tool declaring %s must register", b)
	}
}

// newTestRegistry builds a registry holding one tool of each behaviour plus a
// local tool, for the registry-view tests below.
func newTestRegistry(t *testing.T) *Registry[testContainer] {
	t.Helper()
	r := NewRegistry[testContainer]()
	r.Register(Tool[testContainer]{
		Name:             "query_devices",
		Behaviour:        ReadOnly,
		ServiceKey:       "core-metadata",
		VisibilityRoutes: []Route{ServiceRoute("core-metadata", "/api/v3/device/all", http.MethodGet)},
		Add:              noopAdd,
	})
	r.Register(Tool[testContainer]{
		Name:       "manage_device",
		Behaviour:  Destructive,
		ServiceKey: "core-metadata",
		VisibilityRoutes: []Route{
			ServiceRoute("core-metadata", "/api/v3/device", http.MethodPost),
			ServiceRoute("core-metadata", "/api/v3/device", http.MethodPatch),
		},
		Add: noopAdd,
	})
	r.Register(Tool[testContainer]{
		Name:             "issue_get_command",
		Behaviour:        Additive,
		ServiceKey:       "core-command",
		VisibilityRoutes: []Route{ServiceRoute("core-command", "/api/v3/device/name", http.MethodGet)},
		Add:              noopAdd,
	})
	r.Register(Tool[testContainer]{Name: "search_guidance", Local: true, Behaviour: ReadOnly, Add: noopAdd})
	return r
}

// AnnotationsFor() is what reaches the wire. It must reflect the declaration and
// never leave DestructiveHint nil, because nil is read as the protocol default.
func TestAnnotations_BuiltFromDeclaration(t *testing.T) {
	r := newTestRegistry(t)

	ro := r.AnnotationsFor("query_devices")
	require.NotNil(t, ro)
	assert.True(t, ro.ReadOnlyHint, "query_devices is read-only")
	require.NotNil(t, ro.DestructiveHint, "nil DestructiveHint is read as the protocol default, true")
	assert.False(t, *ro.DestructiveHint)

	dx := r.AnnotationsFor("manage_device")
	require.NotNil(t, dx)
	assert.False(t, dx.ReadOnlyHint)
	require.NotNil(t, dx.DestructiveHint)
	assert.True(t, *dx.DestructiveHint, "manage_device changes state")

	// The third combination, and the reason Behaviour is not two booleans: not
	// read-only, and not destructive, because it only ever adds.
	add := r.AnnotationsFor("issue_get_command")
	require.NotNil(t, add)
	assert.False(t, add.ReadOnlyHint)
	require.NotNil(t, add.DestructiveHint)
	assert.False(t, *add.DestructiveHint)
}

func TestAnnotationsFor_PanicsOnUnregisteredTool(t *testing.T) {
	r := NewRegistry[testContainer]()

	var got any
	func() {
		defer func() { got = recover() }()
		r.AnnotationsFor("zz_never_registered")
	}()

	require.NotNil(t, got)
	assert.Contains(t, got, "unregistered tool")
}

// Every tool is either mapped to upstream routes or explicitly local, and the
// two sets do not overlap in any of the registry's views.
func TestRegistry_MappedAndLocalViews(t *testing.T) {
	r := newTestRegistry(t)

	routes := r.Routes()
	assert.Empty(t, routes["search_guidance"], "local tool must not appear in Routes()")
	assert.NotEmpty(t, routes["query_devices"])
	assert.NotEmpty(t, routes["manage_device"])
	assert.NotEmpty(t, routes["issue_get_command"])

	assert.True(t, r.IsLocal("search_guidance"))
	assert.False(t, r.IsLocal("query_devices"))
	assert.False(t, r.IsLocal("zz_never_registered"), "an unregistered tool must not read as local")

	mapped := r.MappedTools()
	assert.Equal(t, map[string]bool{
		"query_devices":     true,
		"manage_device":     true,
		"issue_get_command": true,
	}, mapped, "MappedTools must hold exactly the upstream-mapped tools")
}

// All() is ordered by name so the tools/list order does not depend on
// registration order.
func TestRegistry_AllIsOrderedByName(t *testing.T) {
	r := newTestRegistry(t)

	names := make([]string, 0)
	for _, tl := range r.All() {
		names = append(names, tl.Name)
	}
	assert.Equal(t, []string{"issue_get_command", "manage_device", "query_devices", "search_guidance"}, names)
}

func TestServiceRoute_PrefixesServiceKey(t *testing.T) {
	got := ServiceRoute("core-metadata", "/api/v3/device/all", http.MethodGet)
	assert.Equal(t, Route{URI: "/core-metadata/api/v3/device/all", Method: http.MethodGet}, got)
}
