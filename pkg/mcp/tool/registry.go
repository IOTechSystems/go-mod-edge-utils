//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package tool is the registration framework for an MCP service's tools: each
// tool declares, in one place, its name, the upstream routes its arguments can
// reach, its behaviour hint, and how it attaches to the server.
//
// The layout is the point. A tool spread across several packages — name
// constant, MCP declaration, route derivation, application logic — means adding
// or changing one requires finding all of them, and the copies can disagree.
// Everything a tool decides lives in one declaration.
//
// The registry is generic over the service's dependency container type C, so
// this package depends on no particular DI framework: each service instantiates
// a Registry with its own container type and keeps thin, service-specific route
// constructors around ServiceRoute.
package tool

import (
	"fmt"
	"slices"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Route is the (URI, method) pair security-proxy-auth's RBAC is keyed by. The URI
// is the upstream path prefixed by its service, as proxy-auth sees it.
type Route struct {
	URI    string
	Method string
}

// ServiceRoute builds a Route for one upstream service, so a tool names the API
// route it reaches without repeating the service prefix. serviceKey is the bare
// service key (e.g. "core-metadata"); each MCP service defines its own thin
// constructors over this.
func ServiceRoute(serviceKey, apiRoute, method string) Route {
	return Route{URI: "/" + serviceKey + apiRoute, Method: method}
}

// Behaviour is what a tool does to the state on the other side of it, and the
// hint clients gate approval on.
//
// One declaration rather than a boolean per protocol hint. ReadOnlyHint and
// DestructiveHint have three meaningful combinations, not four: a pair of
// booleans lets a tool declare the impossible fourth, and lets it declare
// nothing at all.
type Behaviour int

const (
	// The zero value is deliberately not a behaviour, and deliberately unnamed:
	// a tool that declares none fails registration rather than inheriting the
	// protocol default.
	_ Behaviour = iota
	// ReadOnly does not change the state on the other side at all.
	ReadOnly
	// Additive changes it, but only by adding — the protocol's own word for
	// destructiveHint: false on a tool that is not read-only. Reach for it when
	// the upstream route is a GET but the call still leaves something behind.
	Additive
	// Destructive may update or remove what is already there.
	Destructive
)

// String names the behaviour, so a test or a panic quotes the declaration rather
// than an integer.
func (b Behaviour) String() string {
	switch b {
	case ReadOnly:
		return "ReadOnly"
	case Additive:
		return "Additive"
	case Destructive:
		return "Destructive"
	default:
		return fmt.Sprintf("Behaviour(%d)", int(b))
	}
}

// Tool is what the server needs to know about one MCP tool that is not the
// tool's own behaviour. C is the service's dependency container type.
type Tool[C any] struct {
	// Name is the MCP tool name.
	Name string
	// ServiceKey is the upstream service whose client must exist before this
	// tool can be registered. Empty for a tool served entirely in-process.
	ServiceKey string
	// VisibilityRoutes is every upstream route this tool's arguments can reach.
	// tools/list has no arguments, so it decides visibility against the whole set.
	//
	// Required unless Local. It must be DECLARED, never derived from "this tool
	// has no routes": an empty universe is fail-closed, so a forgotten
	// declaration hides the tool — whereas deriving "no routes means local" would
	// turn the same mistake into a tool visible to everyone.
	VisibilityRoutes []Route
	// Local marks a tool served entirely inside the MCP service, with no
	// upstream route to authorize. Explicit, for the reason above.
	Local bool
	// Behaviour is the tool's MCP behaviour hint.
	//
	// Declared here rather than at the mcp.AddTool site because omitting it there
	// is silent and wrong in the dangerous direction: the SDK documents
	// DestructiveHint as "Default: true", so a tool that says nothing is
	// advertised to clients as destructive — and clients gate approval on exactly
	// that. Register refuses the omission.
	Behaviour Behaviour
	// Add attaches the tool to the server.
	Add func(*mcp.Server, C)
}

// Registry holds one MCP service's tool declarations. Each service creates one
// (typically as a package-level variable filled by each tool file's init) via
// NewRegistry.
type Registry[C any] struct {
	tools map[string]Tool[C]
}

// NewRegistry returns an empty Registry.
func NewRegistry[C any]() *Registry[C] {
	return &Registry[C]{tools: map[string]Tool[C]{}}
}

// Register records one tool. Every failure here is a configuration bug, so it
// panics: the service not starting is the loudest possible signal, and unlike a
// test it cannot be skipped or forgotten.
func (r *Registry[C]) Register(t Tool[C]) {
	if t.Name == "" {
		panic("tool: registered with an empty name")
	}
	if t.Add == nil {
		panic(fmt.Sprintf("tool: %q registered with no Add function", t.Name))
	}
	if _, dup := r.tools[t.Name]; dup {
		panic(fmt.Sprintf("tool: duplicate registration for %q", t.Name))
	}

	// The declaration this whole layout exists to make mandatory. A tool with
	// neither a route universe nor an explicit Local marking is invisible to
	// every user, and nothing else in the system would report it.
	switch {
	case t.Local && len(t.VisibilityRoutes) > 0:
		panic(fmt.Sprintf("tool: %q is marked local but declares upstream routes", t.Name))
	case !t.Local && len(t.VisibilityRoutes) == 0:
		panic(fmt.Sprintf("tool: %q declares no visibility routes and is not marked local", t.Name))
	}

	// ServiceKey says which upstream service must be up before this tool loads. A
	// mapped tool without one skips that check and loads even when its service is
	// down; a local tool with one waits on a service it never uses.
	switch {
	case t.Local && t.ServiceKey != "":
		panic(fmt.Sprintf("tool: %q is marked local but declares service key %q", t.Name, t.ServiceKey))
	case !t.Local && t.ServiceKey == "":
		panic(fmt.Sprintf("tool: %q declares no service key and is not marked local", t.Name))
	}

	switch t.Behaviour {
	case ReadOnly, Additive, Destructive:
	default:
		panic(fmt.Sprintf("tool: %q must declare a Behaviour of ReadOnly, Additive or Destructive, not %s",
			t.Name, t.Behaviour))
	}

	r.tools[t.Name] = t
}

// AnnotationsFor builds one tool's MCP annotations from its registration. Central,
// so no tool file can omit them or contradict what it declared, and so
// DestructiveHint is never left nil — nil is read as the protocol default, true.
func (r *Registry[C]) AnnotationsFor(name string) *mcp.ToolAnnotations {
	t, ok := r.tools[name]
	if !ok {
		panic(fmt.Sprintf("tool: annotations requested for unregistered tool %q", name))
	}
	destructive := t.Behaviour == Destructive
	return &mcp.ToolAnnotations{ReadOnlyHint: t.Behaviour == ReadOnly, DestructiveHint: &destructive}
}

// Routes returns every upstream-mapped tool's route universe. Local tools have no
// upstream route and are absent. Sole source of truth for tools/list visibility.
func (r *Registry[C]) Routes() map[string][]Route {
	out := make(map[string][]Route, len(r.tools))
	for name, t := range r.tools {
		if t.Local {
			continue
		}
		out[name] = slices.Clone(t.VisibilityRoutes)
	}
	return out
}

// IsLocal reports whether a tool is served entirely inside the MCP service and
// so has no upstream route to authorize.
func (r *Registry[C]) IsLocal(name string) bool { return r.tools[name].Local }

// MappedTools returns every tool name that reaches an upstream route — the
// authoritative set for RBAC and for server-instruction guards.
func (r *Registry[C]) MappedTools() map[string]bool {
	out := make(map[string]bool, len(r.tools))
	for name, t := range r.tools {
		if !t.Local {
			out[name] = true
		}
	}
	return out
}

// All returns every registered tool, ordered by name so registration order — and
// therefore the tools/list order — does not depend on file or init order.
func (r *Registry[C]) All() []Tool[C] {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Tool[C], 0, len(names))
	for _, name := range names {
		out = append(out, r.tools[name])
	}
	return out
}
