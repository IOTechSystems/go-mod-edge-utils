# sse/progress — Publisher-driven async-progress SSE

`progress.Tracker` is the publisher-driven sibling of [`pkg/sse`](../). Use it when an HTTP-triggered async operation has a definite terminal (success or failure) and the client wants to observe progress over an SSE stream that opens slightly before, during, or shortly after the job runs.

## Scope — when to use this package

This package is designed for **bounded async operations with a terminal**, emitting **monotonic snapshot events**:

- A trigger starts the work (typically an HTTP POST).
- Work emits a sequence of progress events ending in `Job.Finish`.
- Each event is a snapshot — later events subsume earlier ones (e.g. `progress=70%` carries everything `progress=30%` did; `result={count: 10, items: [...]}` carries everything `result={count: 5, ...}` did).
- Subscribers observe events from any point in time; late subscribers see the **latest cached event** then live tail.

Memory cost is O(1) per Job — only the most recent payload is held. Retention drops the Job after the terminal window expires.

**Cache-last vs full-log replay.** Earlier revisions retained the full event log so late subscribers saw the complete history. The current design caches only the last event because the in-use cases (discovery progress, auto-tagging results) emit cumulative snapshots where intermediate events are redundant. Caching one event eliminates the replay-window event-drop race and bounds memory cost. **Delta-style events** — e.g. emitting `"found device A"` and `"found device B"` as separate events that don't include the cumulative list — are not supported; encode such state into a snapshot payload instead, or use `pkg/sse.Manager` with caller-side history.

**Do NOT use this package for:**

| Scenario | Use instead |
| --- | --- |
| High-volume event streams (MQTT bus relay, telemetry) | `pkg/sse.Manager` |
| Subscriber-driven polling without a terminal (lists, dashboards, logs) | `pkg/sse.NewPolling` |
| Delta-style events where every payload carries unique information | rethink — either coalesce into snapshots, or use `pkg/sse.Manager` with caller-side history |

## Quick start

```go
import (
    "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/sse/progress"
)

// At service startup, one Tracker per progress-style flow.
tracker := progress.New(serviceCtx, 30*time.Second, 30*time.Second, lc)

// On the POST that triggers the work:
job, isNew := tracker.Start("discovery/" + edgeInst + "/" + service)
if !isNew {
    // Coalesce: another trigger is already running on this topic; the
    // caller typically returns 200 to signal "joined existing".
    return ok200(c)
}

go func() {
    // Safety-net terminal in case work returns early without calling
    // Finish (idempotent — the happy-path Finish below wins if called).
    defer job.Finish(progressFailed(errors.New("job did not complete")))

    if err := doWork(ctx, job); err != nil {
        job.Finish(progressFailed(err))
        return
    }
    job.Finish(progressDone())
}()
return accepted202(c)

// On the GET that streams progress:
func (c controller) listen(ec echo.Context) error {
    err := tracker.Subscribe(ec, topic)
    if errors.Is(err, progress.ErrNoTopic) {
        return ec.JSON(http.StatusNotFound, ...)
    }
    return err
}

// doWork emits progress directly on the Job — no callback indirection.
func doWork(ctx context.Context, job *progress.Job) error {
    job.Publish(progressStarted())
    // ... work ...
    return nil
}
```

## Two entry points: Start vs StartOrJoin

Both atomically return `(*Job, isNew bool)`. The difference is what happens when the topic has a **retained terminal** entry:

| Entry point | Running entry | Retained terminal entry | No entry |
| --- | --- | --- | --- |
| `Start` | Join (isNew=false) | **Discard and create fresh** (isNew=true) | Create (isNew=true) |
| `StartOrJoin` | Join (isNew=false) | **Reuse retained** (isNew=false) | Create (isNew=true) |

Selection rule:

- **Trigger-driven flows** (user clicks "Start scan", HTTP POST) → `Start`. Each trigger conceptually starts a new run; the retained terminal from a previous run must not be observed as if it were the new run's result.
- **Subscribe-driven flows** ("compute the tag-scan result once; many dashboards subscribe to it") → `StartOrJoin`. Late subscribers should see the cached terminal, not retrigger the work.

Both methods are atomic over `Tracker.mu`; concurrent callers on the same topic see exactly one `isNew=true` and the rest coalesce.

## Lifecycle

```
        (no entry)
             |
             | Start / StartOrJoin (isNew=true)
             v
        +---------+
        | Running |  -- Publish (updates last + fans out latest-wins)
        +---------+
             |
             | Finish (terminal, idempotent)
             v
        +----------+
        | Retained |  -- late subscribers within retention see the
        +----------+     cached terminal and close.
             |
             | cleanup (retention expires OR tracker ctx cancelled)
             v
        (no entry)
```

`Start` on a retained entry creates a fresh Job, replacing the map entry. The previous Job's cleanup goroutine sees the identity check fail (`jobs[topic] != prevJob`) and leaves the new entry alone.

## API

| Symbol | Role |
| --- | --- |
| `New(ctx, retention, heartbeat, lc)` | Construct a Tracker; zero/negative durations use 30s defaults. |
| `Tracker.Start(topic)` | Trigger-driven entry. Returns `(*Job, isNew bool)`. |
| `Tracker.StartOrJoin(topic)` | Subscriber-driven entry. Reuses retained. Returns `(*Job, isNew bool)`. |
| `Tracker.Subscribe(c, topic)` | Attach an SSE subscriber. Writes the cached last event then live tail. Returns `ErrNoTopic` if topic is unknown. |
| `Job.Publish(data)` | Update cached last + fan out (latest-wins). No-op once terminal. |
| `Job.Finish(data)` | Cache terminal + close subscribers + start retention. Idempotent. |
| `ErrNoTopic` | Sentinel: no active or recent Job for topic. |

## Runtime requirement

This package (and the parent `pkg/sse`) **requires** the `ResponseWriter` chain to expose `SetWriteDeadline` via `http.ResponseController` — directly or through `Unwrap()`. It is the only standard way Go offers to bound a stuck `Write` on a slow/dead client.

Stock Echo + `net/http` meets this out of the box. Custom middleware that wraps the writer MUST forward `Unwrap()`. If the requirement is not met:

- `WriteSSEHeaders` logs `Errorf` once at stream start naming the missing capability
- The first event or heartbeat write returns an error and the handler exits cleanly (200 OK, no data)

Fail-loud is intentional. Tolerating a missing deadline silently disables slow-client protection — a slow reader could otherwise keep a server-side goroutine blocked in `fmt.Fprintf` indefinitely, since no in-Go mechanism can cancel that write without `SetWriteDeadline`.

Tests use a fixture that implements `SetWriteDeadline` as a no-op (see `flushableRecorder` in `subscribe_test.go`).

> **Note**: no current deployment of this package uses middleware that breaks `ResponseController` — every active call site sits on stock Echo. This section is recorded as a forward reference: if some future integration ever returns 200-then-immediately-close on its first SSE request, the probe log here is the first thing to check, and the path forward is either to fix the offending middleware to forward `Unwrap()` or to introduce an explicit fallback (e.g. `goroutine + select` timeout) in `pkg/sse/utils.go`.

## What this package does NOT do

- **Mutual exclusion across calls** — `Start` coalesces concurrent same-topic triggers, but cross-topic mutual exclusion (e.g. profile scan blocking discovery on the same edge instance) belongs in the caller. Use a per-resource lock alongside the tracker.
- **De-duplication of payloads** — emit at whatever cadence your work loop generates. A subscriber may briefly observe the cached event echoed on the wire if a same-value `Publish` lands between attach and the live loop's first iteration; harmless for idempotent progress payloads.
- **Full event history replay** — late subscribers receive only the latest cached event. Design events as monotonic snapshots (see "Scope" above). The previous full-log design was reverted because its replay window introduced an event-drop race with no benefit for snapshot-style payloads.

## See also

- Parent package: [`../`](../) — `Manager` / `broadcaster` / `PollingService`.
- The internal lock model is `Tracker.mu` → `Job.mu`, never reversed. `Publish` / `Finish` touch only `Job.mu`; `Start` / `StartOrJoin` / cleanup touch only `Tracker.mu` for the brief map mutation (with `isTerminal()` on a retained entry the only Job.mu acquisition under Tracker.mu, matching the lock order). Cleanup goroutines wake on `j.finished` or `tracker.ctx.Done()`; deletion is gated by an identity check so a replacement Job survives an orphaned old Job's cleanup pass.
