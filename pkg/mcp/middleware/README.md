# Receiving middleware

The MCP receiving chain: what a caller is allowed to *do* (`tools/call`) and what
they are allowed to *see* (`tools/list`). Both delegate every decision to
security-proxy-auth — the MCP service never evaluates policy itself.

This package holds the mechanisms: argument decoding (`DecodeArguments`), request
logging (`Logging`), tools/call authorization (`Auth`), tools/list visibility
filtering (`Visibility`) and the whole-call deadline (`Deadline`). The installing
service resolves their dependencies and fixes the order they install in
(central-mcp's `internal/controller/middleware` is the reference wiring), under
two positional contracts:

- `Deadline` must nest **outermost**, the only position it works from: its
  `context.WithTimeout` has to be on the context every layer below uses.
- `DecodeArguments` must nest **innermost** — see below.

### What the ceiling bounds

`ToolCallCeiling` (30 s, `deadline.go`) covers the authorization sub-call, the
upstream request and its body, on `tools/call` and on `tools/list` — the
proxy-auth `AuthRoutes` batch rides on the latter.

⚠ **It is not the only bound underneath.** `tools/call` narrows the proxy-auth
call to `authz.authorizationTimeout` (5 s, strictly under this one, with the
measurement beside it); `tools/list` does not, because its `AuthRoutes` call is
the last thing on that path and has no budget after it to protect.

The 30 s comes from a measurement: for a measured core-data call the wait for the
response to *begin* is ~99.9% of the call. ⚠ It bounds the **body** as well as the
headers, so headers at 28 s followed by a 10 s body exceeds it. Not configurable:
nothing has hit it, and a knob can be set wrong.

⚠ One layer deliberately sits outside it. `decode_arguments` reads the schemas back
under `context.WithoutCancel`: that readback happens once for the process and must
not be latched empty by one caller's timeout.

### Whose deadline ended the call

Several layers react to a cancelled context and every one needs the same
distinction: **our ceiling expired while the caller waited** (the other side did
not answer — an outage, worth a log line, invalidates a cached address) versus
**the caller stopped waiting** (evidence about nobody, and it must page no one).

`ctx.Err()` cannot tell them apart — our ceiling and a caller's own deadline are
both `context.DeadlineExceeded` — so `deadline.go` stamps
`callstate.ErrCeilingExpired` as the context's *cause*, and `callstate.Abandoned`
is the only place that comparison is written. Each site below carries only its own
consequence; the mechanism lives in `pkg/mcp/callstate`.

| asks | so that |
|---|---|
| `pkg/mcp/authz/authz.go` | an abandoned call is not reported as a proxy-auth outage |
| `middleware/visibility.go` | a hang-up during `tools/list` authorization is not one either |
| `pkg/mcp/rs/challenge.go` | a client that hangs up mid-introspection is not a 503 |

The installing service adds its own sites — central-mcp's
`injector/invalidate.go` and `tool/ping_service.go` carry two more; see its
middleware README.

⚠ **A test that simulates the ceiling must stamp the cause.** A bare
`context.WithTimeout` is an *unstamped* expiry, which by definition belongs to the
caller — so it exercises the hang-up branch while claiming to exercise this one.
Two tests did exactly that and passed by accident.

⚠ **`decode_arguments` must be innermost.** The gate is the next thing after it,
so one layer further out the gate would judge the original argument and reject the
call. Its own `tools/list` would also be filtered by `visibility` into a
per-caller subset and logged by `logging` as a request no client made.

`visibility` also sets `cacheScope: "private"` on the list it filters. Keeping
that on the same result, in the same place, is what stops the declaration and the
filtering from drifting apart: a separate layer that rebuilt the
`ListToolsResult` would decide for itself which tools are on it, and could put
back what `visibility` removed.

## Route universes: one declaration, two consumers

proxy-auth's RBAC is keyed by `(URI, method)` — an Edge Central REST endpoint, not
a tool name. So every tool must be expressed as routes before it can be
authorized.

Every tool declares its **route universe** in its own file in the installing
service's tool package — every route its arguments can reach. `pkg/mcp/tool` is
the registration framework; central-mcp's middleware README shows a declaration.

Only `tools/list` reads this, via `tool.Routes()`. It has no arguments, so it
cannot know which single route a call would take, and must decide visibility
against the whole set.

`tools/call` needs no route at all. It used to resolve one from the arguments and
authorize that, which made the resolved route and the dispatched route two copies
of one decision; authorization now happens in `pkg/mcp/authz`, inside the
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

A tool with no upstream route to authorize — route-authz cannot speak to it — sets
`Local: true` in its own file and stays visible, resting on endpoint-level
`bearerAuthn` having required a valid token. The marking is declared, never
inferred — see the panic above. Which tools are Local, and why nothing authorizes
each of them, is the installing service's to document — see central-mcp's
middleware README for its three.

⚠ **A Local tool may call an upstream.** That matters because `authz.Transport`
authorizes `ServicePrefix + req.URL.EscapedPath()` of the request actually being
sent, which is what makes every route-mapped tool immune to a caller splicing path
segments into an upstream URL. A Local tool has no such backstop, so **any
caller-controlled value it puts in an upstream path must be validated by the tool
itself** — central-mcp's `ping_service` does that with a positive character set
(`internal/tool/ping_service.go`, `isOneServiceName`). A Local tool that calls an
upstream without that validation is exploitable by any caller holding a token.

For the same reason such a tool must not pass an upstream error back verbatim: it
carries the upstream's own host and port, and on a 5xx its response body, to a
caller no route authorization ran on. Log the cause, return a message that names
only the upstream and the argument.

### Fail-closed

Every failure path yields no list rather than an unfiltered one:

| Situation | Result |
|---|---|
| proxy-auth not in configuration | every `tools/list` rejected (`RejectToolsList`) |
| no `Authorization` header | `Unauthorized` |
| token invalid/expired (`401`) | `Unauthorized` |
| outage / `5xx` / `404` / timeout | `ServerError` — an outage is not a decision |
| tool with no universe and not local | dropped |

The error returned to the caller is deliberately generic; the client's real error
is logged instead, so an upstream `404` on the batch endpoint stays diagnosable
without leaking proxy-auth internals over the MCP protocol.

## Why not the go-mod-central-ext client

`AuthClient.Auth`/`AuthRoutes` hardcode the first-party `/api/v3/auth` and
`/api/v3/auth-routes` paths, which reject OAuth tokens. Both clients — this
package's `NewAuthRoutesClient` and `pkg/mcp/authz`'s — are hand-rolled against
the `/api/v3/oauth/*` equivalents, and take an `AuthenticationInjector` from the
installing service; that injector must stamp nothing (central-mcp's
`passthroughInjector`) so the service's own JWT never overwrites the forwarded
end-user bearer.

## Known limitations

- **No `notifications/tools/list_changed`.** A permission change mid-session is
  not pushed; the client sees it on its next `tools/list`. This is a staleness
  issue, not a security one — nothing here is cached, and `rbac` re-authorizes
  every `tools/call`, so a revocation takes effect immediately on execution even
  while a client still shows the tool. "Nothing is cached" is a claim about the
  `ttlMs: 0` the service ships, not about the protocol: set a positive `ttlMs`
  and the staleness window becomes exactly that long — which is why the list is
  declared `private` now rather than when someone reaches for the speed-up.
- **Filtering does not refill a page.** Safe only while the whole surface fits in
  one page — the SDK's `DefaultPageSize` is 1000. Set a smaller `PageSize`, or
  grow past it, and a caller whose first page is entirely denied gets an empty
  `Tools` with a non-empty `NextCursor`.
