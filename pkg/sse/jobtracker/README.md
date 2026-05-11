# README #

Tracks the lifecycle of in-process jobs that publish progress over Server-Sent
Events. Use this when a `POST` triggers a job whose progress is streamed back
through a separate `GET /sse...` request, and a late-connecting client must
still receive the final event even if the job already finished.

## How To Use ##

Import the package:

```go
import "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/sse/jobtracker"
```

Define the wire envelope your API sends to clients — the same shape is used for
both live events and the retained terminal event:

```go
type Envelope struct {
    Details ProgressEvent `json:"details"`
}
type ProgressEvent struct {
    Progress int    `json:"progress"`         // 0 = started, 100 = ok, -1 = error
    Message  string `json:"message,omitempty"`
}
```

Construct the tracker once at startup. `ctx` cancellation triggers cleanup of
any pending retention timers. `lc` is used for debug-level logging; pass `nil`
to disable. `retention` is how long a finished job's terminal payload is held
for late subscribers; `heartbeat` is the live-stream keep-alive interval (also
used as the per-write deadline). Both must be positive — the constructor
panics otherwise:

```go
tracker := jobtracker.New(ctx, 30*time.Second, 30*time.Second, lc)
```

Note: jobtracker is independent of `sse.Manager`. Live SSE handlers backed by
the manager (live dashboards, polling streams) and jobtracker-backed handlers
can coexist in the same service without sharing any broadcaster state.

### Trigger handler (publisher side) ###

```go
func TriggerJob(c echo.Context) error {
    topic := "discovery/" + c.Param("name")

    // jobtracker does not enforce mutual exclusion. Caller serializes
    // concurrent jobs on the same topic with their own lock.
    release, ok := acquireLock(topic)
    if !ok {
        return c.JSON(http.StatusConflict, ...)
    }

    job, replaced := tracker.StartJob(topic)
    if replaced {
        // External serialization let through a duplicate; investigate.
        lc.Warnf("jobtracker: replaced an in-flight or retained entry for %s", topic)
    }

    go func() {
        defer release()
        // Safety-net Finish: if runJob neither finishes nor panics with a
        // terminal payload, ensure the entry is still closed so subscribers
        // do not hang. Finish is idempotent, so the explicit call inside
        // runJob (if any) wins.
        defer job.Finish(Envelope{Details: ProgressEvent{Progress: -1, Message: "job did not complete"}})
        defer func() {
            if r := recover(); r != nil {
                job.Finish(Envelope{Details: ProgressEvent{Progress: -1, Message: fmt.Sprint(r)}})
            }
        }()

        runJob(func(progress int, msg string) {
            event := Envelope{Details: ProgressEvent{Progress: progress, Message: msg}}
            if progress == 100 || progress == -1 {
                job.Finish(event) // broadcasts AND retains
            } else {
                job.Publish(event) // live broadcast only
            }
        })
    }()

    return c.NoContent(http.StatusAccepted)
}
```

### Listen handler (subscriber side) ###

```go
func ListenJob(c echo.Context) error {
    topic := "discovery/" + c.Param("name")

    err := tracker.Stream(c, topic)
    if errors.Is(err, jobtracker.ErrNoJob) {
        return c.JSON(http.StatusNotFound, ...)
    }
    return err
}
```

That is the full integration. Everything below is reference material.

## What `Stream` Does ##

`Stream` dispatches on the topic's current state:

| State | What `Stream` does |
|---|---|
| Never started, or already cleaned up | Returns `ErrNoJob` without writing anything |
| Job finished within retention window | Writes the retained terminal payload and returns `nil` |
| Job is running | Streams events until the job ends or the client disconnects |

The publisher uses `Job.Publish` for non-terminal events and `Job.Finish` for
the last event. `Finish` both broadcasts the payload to live subscribers and
retains it as the terminal — live and replayed events are byte-for-byte
identical to the client.

After `Job.Finish` is called, the entry is retained for the duration passed to
`New` so a subscriber that connects shortly after completion still sees the
terminal event. After the window expires, the entry is removed and subsequent
`Stream` calls return `ErrNoJob`.

`Finish` is idempotent: only the first call per `Job` has any effect, so a
deferred safety-net `Finish` and an explicit happy-path `Finish` can coexist.

If you know the job will not produce a meaningful terminal — e.g. the worker
goroutine failed to spawn, or the operation was cancelled before any progress
was reported — call `Job.Abort` instead of `Finish`. Abort removes the entry
immediately (no retention) and releases live subscribers without broadcasting
or retaining any payload. It shares the once-guard with `Finish`, so only the
first of `Finish`/`Abort` per `Job` takes effect.

## When Not To Use This ##

Reach for `sse.Manager` directly when the stream exists independently of any
single client request — a live dashboard, a polling service, a message-bus tap
that's always running. `jobtracker` is for the specific case where the trigger
and the subscription are two separate HTTP requests and the job is short-lived.

If your progress already comes from a message bus or another service, you
likely don't need `jobtracker` either — the bus absorbs the timing race that
`jobtracker`'s retention is solving.

## Concurrency ##

`Tracker` is goroutine-safe but does **not** enforce mutual exclusion between
concurrent jobs on the same topic. Two concurrent `StartJob` calls on the same
topic will overwrite each other's entry. Callers that need at most one job per
topic must serialize externally with a per-topic lock; `StartJob`'s second
return value (`replaced`) flags when external serialization missed a beat.

A `Job` handle is bound to the entry it was created for. If a newer `StartJob`
replaces the entry while the original `Job` is still running, the original
`Job.Finish` becomes a no-op — it will not corrupt the new entry's state.

The retention timer is similarly entry-scoped: a `StartJob` called during a
previous job's retention window replaces the entry, and the prior finish's
cleanup goroutine compares entries by identity and skips removal.

There is a narrow window where a subscriber that calls `Stream` at the exact
moment `Job.Finish` runs may race past the terminal-replay branch (it sees
`TerminalSet == false`) and then have its live subscription stopped immediately by
the `finished` close. Two sub-cases:

- If `Subscribe` happens **before** `Finish` publishes the terminal, the
  live-stream's drain loop ensures the buffered terminal event is delivered
  before the handler returns.
- If `Subscribe` happens **after** that publish (the window between publish
  and `close(finished)`), the subscriber was not on the entry's roster when
  the broadcast fired and will receive nothing.

Standard SSE clients reconnect after the handler returns, and the second
attempt takes the terminal-replay path. Publishers should not rely on a single
connection observing the terminal during this race.

When the `ctx` passed to `New` is cancelled, every pending retention timer
wakes early and runs its cleanup branch — entries are deleted from the tracker
map — so no entries leak across shutdown.

## Low-Level API ##

If `Stream` doesn't fit, the underlying primitives are public:

```go
job, replaced := tracker.StartJob(topic)    // → Job handle + flag if an entry was replaced
job.Publish(payload)                         // → live event to current subscribers
job.Finish(payload)                          // → broadcast + retain as terminal, close + start retention; idempotent
job.Abort()                                  // → discard entry without broadcast/retain; shares once-guard with Finish

state, ok := tracker.LookupJob(topic)        // → manual state inspection
```

For one-shot SSE responses (e.g. replaying a retained terminal yourself), use
`sse.SetHeaders` and `sse.WriteEvent` directly.

`LookupJob` returns `(*JobState, bool)`:

- `(nil, false)` — no active or recent job
- `(state, true)` with `state.TerminalSet == false` — running; watch
  `state.Finished` for end-of-job
- `(state, true)` with `state.TerminalSet == true` — finished within retention;
  `state.Finished` is already closed. `state.Terminal` holds whatever the
  publisher passed to `Job.Finish` (may be `nil` if `Finish(nil)` was called).
  Type-assert `state.Terminal` to your payload type when non-nil.

`JobState.Subscribe` / `JobState.Unsubscribe` let tests and advanced consumers
attach a buffered channel to receive live events directly. Unsubscribe closes
the returned channel; calling it on a channel that is not subscribed (or
already unsubscribed) is a no-op. Channels with no room drop events rather
than block, mirroring how `Stream` handles slow clients.
