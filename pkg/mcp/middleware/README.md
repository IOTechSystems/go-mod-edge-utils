# Receiving middleware

The MCP receiving chain: what a caller is allowed to *do* (`tools/call`) and what
they are allowed to *see* (`tools/list`). Both delegate every decision to
security-proxy-auth — central-mcp never evaluates policy itself.

```
                    ┌──────────────────────────────────┐
   /mcp request ──▶ │  visibility   tools/list filter   │  POST /oauth/auth-routes
                    │  rbac         tools/call authz    │  POST /oauth/auth
                    │  logging                          │
                    └────────────────┬─────────────────┘
                                     ▼
                              tool handler
```

`registry.go` owns the order. `Chain` is applied outermost-first, so the runtime
nesting is `visibility(rbac(logging(handler)))` — irrelevant to behaviour, since
each middleware acts on exactly one method and passes everything else through.

`visibility` also sets `cacheScope: "private"` on the list it filters. Keeping
that on the same result, in the same place, is what stops the declaration and the
filtering from drifting apart: a separate layer that rebuilt the
`ListToolsResult` would decide for itself which tools are on it, and could put
back what `visibility` removed.

## Route universes: one declaration, two consumers

proxy-auth's RBAC is keyed by `(URI, method)` — an Edge Central REST endpoint, not
a tool name. So every tool must be expressed as routes before it can be
authorized.

Every tool declares its **route universe** in its own file in `internal/tool` —
every route its arguments can reach. A 1:1 tool declares one; `manage_device_profile`
declares 11:

```go
register(Tool{
    Name:       NameManageDevice,
    ServiceKey: coreCommon.CoreMetaDataServiceKey,
    VisibilityRoutes: []Route{
        MetadataRoute(coreCommon.ApiDeviceRoute, http.MethodPost),
        MetadataRoute(coreCommon.ApiDeviceRoute, http.MethodPatch),
        MetadataRoute(coreCommon.ApiDeviceByNameRoute, http.MethodDelete),
    },
    Add: addManageDevice,
})
```

Only `tools/list` reads this, via `tool.Routes()`. It has no arguments, so it
cannot know which single route a call would take, and must decide visibility
against the whole set.

`tools/call` needs no route at all. It used to resolve one from the arguments and
authorize that, which made the resolved route and the dispatched route two copies
of one decision; authorization now happens in `internal/authz`, inside the
transport each upstream client is built with, against the request being sent. So
an inaccurate universe can only show or hide a tool in the catalogue — it can no
longer permit a call.

A tool declaring neither routes nor `Local` **panics at registration**. An empty
universe is fail-closed, so a forgotten declaration would otherwise hide the tool
with nothing reporting it — and deriving "no routes means local" would invert that
into showing it to everyone.

## tools/list filtering

`tools/list` carries no arguments, so there is no single route to authorize
against — visibility is decided against the whole universe.

1. Call `next` to get the real catalogue.
2. Collect every listed tool's universe into one deduplicated batch.
3. One POST to proxy-auth `/api/v3/oauth/auth-routes` — a single introspection
   plus one Casbin `BatchEnforce`. Always answers `200` with a per-route
   `authResult` array; never `204`/`403`.
4. Keep a tool when **any** of its routes is allowed.

### Union, not intersection

A user who may `GET` but not `DELETE` still sees `manage_filters`, because some of
its actions are available to them. An intersection would hide a tool a
partially-privileged user can genuinely use, and the over-reaching action is still
refused per-call by `rbac`. Visibility is a *discovery* filter; `rbac` is the
enforcement boundary. Hiding a tool never grants anything.

### Local tools

`search_guidance` and `get_guidance` are served in-process with no upstream route,
so route-authz cannot speak to them. They set `Local: true` in their own files and
stay visible, resting on endpoint-level `bearerAuthn` having required a valid
token. The marking is declared, never inferred — see the panic above.

`tools/call` needs no exemption for them: they make no upstream call, so nothing
authorizes them in the first place.

### Fail-closed

Every failure path yields no list rather than an unfiltered one:

| Situation | Result |
|---|---|
| proxy-auth not in configuration | every `tools/list` rejected (`rejectToolsList`); startup aborts before this in normal operation |
| no `Authorization` header | `Unauthorized` |
| token invalid/expired (`401`) | `Unauthorized` |
| outage / `5xx` / `404` / timeout | `ServerError` — an outage is not a decision |
| tool with no universe and not local | dropped |

The error returned to the caller is deliberately generic; the client's real error
is logged instead, so an upstream `404` on the batch endpoint stays diagnosable
without leaking proxy-auth internals over the MCP protocol.

`tools/call` has no such fallback to build: with proxy-auth unconfigured the
upstream clients are built with an authorizer that refuses everything, so no call
leaves regardless of how the middleware was constructed.

## Why not the go-mod-central-ext client

`AuthClient.Auth`/`AuthRoutes` hardcode the first-party `/api/v3/auth` and
`/api/v3/auth-routes` paths, which reject OAuth tokens. Both clients here are
hand-rolled against the `/api/v3/oauth/*` equivalents, sharing
`passthroughInjector` so central-mcp's own service JWT never overwrites the
forwarded end-user bearer.

A universe that disagrees with the endpoints a tool actually reaches makes
visibility wrong in either direction, so it is checked exhaustively rather than by
example — and against real traffic, not against a prediction.
`TestEndpointDomains_ActualUpstreamMatchesOracle` in `test/integration` declares
each tool's argument *domain*, walks the full cartesian product (8093
combinations), calls every one of them for real, and asserts that the endpoints a
mock upstream received are exactly the declared universe, in both directions.
Leaving a value out of a domain makes a declared route unreachable, so the check
also guards its own completeness.

## Known limitations

- **No `notifications/tools/list_changed`.** A permission change mid-session is
  not pushed; the client sees it on its next `tools/list`. This is a staleness
  issue, not a security one — nothing here is cached, and `rbac` re-authorizes
  every `tools/call`, so a revocation takes effect immediately on execution even
  while a client still shows the tool. "Nothing is cached" is a claim about the
  `ttlMs: 0` this service ships, not about the protocol: set a positive `ttlMs`
  and the staleness window becomes exactly that long — which is why the list is
  declared `private` now rather than when someone reaches for the speed-up.
- **Filtering does not refill a page.** Safe only while the whole surface fits in
  one page — the SDK's `DefaultPageSize` is 1000 against ~30 tools, and `mcp.go`
  sets no `PageSize`. Set one, or grow past it, and a caller whose first page is
  entirely denied gets an empty `Tools` with a non-empty `NextCursor`.
