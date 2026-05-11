//
// Copyright (C) 2026 IOTech Ltd
//

// Package jobtracker tracks the lifecycle of in-process jobs whose progress is
// streamed back to clients over Server-Sent Events.
//
// The classic case: a POST triggers an async job, and a separate GET /sse...
// streams the progress back. The two requests can connect across time — a
// late-connecting client must still receive the terminal event even if the
// job already finished. jobtracker provides this with a publisher-driven
// lifecycle: an entry is created when StartJob is called and retained for a
// configurable window after Job.Finish so a subscriber connecting shortly
// after completion still receives the terminal event.
//
// Why this lives next to sse.Manager but does not use it:
//
// sse.Manager is subscriber-driven (broadcasters live only while observed,
// fitting live-dashboard / polling use cases). jobtracker is publisher-driven
// (entries live until the publisher says they don't, plus a retention tail).
// Mixing the two on the same primitives produced a pile of subtle race
// conditions, so jobtracker is implemented as a sibling of sse.Manager that
// shares only the wire-format helpers (sse.SetHeaders, sse.WriteEvent), not
// the lifecycle.
package jobtracker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/sse"
)

// ErrNoJob is returned by Stream when no job exists for the given topic, either
// because none has ever started or because it finished and the retention window
// already expired. Callers typically map this to HTTP 404.
var ErrNoJob = errors.New("jobtracker: no active or recent job for topic")

const subscriberBuffer = 64

// Tracker tracks active and recently-finished jobs keyed by topic.
//
// Tracker is unopinionated about concurrency between jobs on the same topic.
// Callers are expected to serialize StartJob/Finish per topic externally
// (e.g. via a per-topic lock); without that, two concurrent StartJob calls on
// the same topic will overwrite each other's entry.
type Tracker struct {
	mu        sync.RWMutex
	shutdown  <-chan struct{}
	lc        log.Logger
	jobs      map[string]*jobEntry
	retention time.Duration
	heartbeat time.Duration
}

// jobEntry is the per-topic record stored in Tracker.jobs. It owns its own
// subscriber set — jobtracker does not delegate broadcasting to sse.Broadcaster
// so that the publisher-driven lifecycle is not entangled with sse.Manager's
// subscriber-driven one.
type jobEntry struct {
	mu          sync.RWMutex
	subscribers map[sse.SubscriberCh]struct{}
	terminal    any           // payload passed to Job.Finish; may be nil
	terminalSet bool          // true once Job.Finish is called; distinguishes Finish(nil) from "still running"
	finished    chan struct{} // closed by Job.Finish or Job.Abort
	finishOnce  sync.Once
}

// JobState is the public, read-only view of a tracked job, returned by LookupJob.
//
// Finished is closed when the job ends. TerminalSet is true once Job.Finish has
// been called; Terminal holds the payload (which may legitimately be nil).
// Callers must type-assert Terminal back to whatever shape the publisher used.
//
// Subscribe / Unsubscribe let tests and advanced consumers observe live events
// without going through Stream. Most callers should use Stream instead.
type JobState struct {
	Terminal    any
	TerminalSet bool // true once Finish is called; use this, not Terminal != nil
	Finished    <-chan struct{}

	entry *jobEntry
}

// Subscribe attaches a buffered channel to receive live events on the job.
// The returned channel is closed by Unsubscribe.
//
// Subscribing to an already-finished job is allowed but useless: no further
// events will be published. Callers that want the terminal payload should
// consult JobState.Terminal directly.
func (s *JobState) Subscribe() sse.SubscriberCh {
	ch := make(sse.SubscriberCh, subscriberBuffer)
	s.entry.mu.Lock()
	s.entry.subscribers[ch] = struct{}{}
	s.entry.mu.Unlock()
	return ch
}

// Unsubscribe detaches and closes the given channel. Calling it on a channel
// that is not subscribed (or already unsubscribed) is a no-op.
func (s *JobState) Unsubscribe(ch sse.SubscriberCh) {
	s.entry.mu.Lock()
	defer s.entry.mu.Unlock()
	if _, ok := s.entry.subscribers[ch]; !ok {
		return
	}
	delete(s.entry.subscribers, ch)
	close(ch)
}

// Job is a handle returned by StartJob. Use Publish during the job and Finish
// when it ends; Finish captures the terminal event and starts the retention timer.
//
// All three fields are needed: tracker+topic locate the entry in the map, and
// entry identifies which generation this Job owns so a stale Finish cannot
// corrupt a replacement entry on the same topic.
type Job struct {
	tracker *Tracker
	topic   string
	entry   *jobEntry
}

// New creates a Tracker that retains finished jobs for retention before
// removing them. heartbeat controls how often a comment line is sent on live
// streams (and is also reused as the per-write deadline so broken peers are
// detected promptly). Both must be positive — non-positive values defeat the
// late-subscriber guarantee or break stream liveness, so this panics on misuse.
//
// When ctx is cancelled, any pending retention timer wakes early and runs its
// cleanup so no entries leak across shutdown.
func New(ctx context.Context, retention, heartbeat time.Duration, lc log.Logger) *Tracker {
	if retention <= 0 {
		panic(fmt.Sprintf("jobtracker.New: retention must be positive, got %s", retention))
	}
	if heartbeat <= 0 {
		panic(fmt.Sprintf("jobtracker.New: heartbeat must be positive, got %s", heartbeat))
	}
	return &Tracker{
		shutdown:  ctx.Done(),
		lc:        lc,
		jobs:      map[string]*jobEntry{},
		retention: retention,
		heartbeat: heartbeat,
	}
}

// StartJob creates (or replaces) the entry for topic and returns a Job handle
// plus a flag indicating whether an existing entry was replaced. The replaced
// flag lets callers detect missed external serialization — a true value means
// either a previous job was still running on this topic or its retention
// window had not yet expired.
func (t *Tracker) StartJob(topic string) (*Job, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, replaced := t.jobs[topic]
	e := &jobEntry{
		subscribers: map[sse.SubscriberCh]struct{}{},
		finished:    make(chan struct{}),
	}
	t.jobs[topic] = e

	if replaced {
		t.debugf("jobtracker: started job %q (replaced previous entry)", topic)
	} else {
		t.debugf("jobtracker: started job %q", topic)
	}
	return &Job{tracker: t, topic: topic, entry: e}, replaced
}

// Publish sends a non-terminal progress event to all currently-connected
// subscribers. Use Finish for the last event of the job — Finish broadcasts AND
// retains, so callers do not need to publish the terminal event themselves.
//
// Publish does not check slot identity. If a stale Job (whose entry was
// replaced by a newer StartJob) calls Publish, the event is sent into the
// stale entry's subscriber set, which is detached from the current map entry
// and will never gain new subscribers. Callers relying on per-topic
// serialization should not see this in practice.
func (j *Job) Publish(payload any) {
	j.entry.publish(payload)
}

// Finish marks the job as complete with the given terminal payload. It does
// three things:
//
//  1. Broadcasts payload to currently-connected subscribers.
//  2. Retains payload as the entry's terminal so subscribers connecting during
//     the retention window still receive it via Stream's replay path.
//  3. Schedules removal of the entry after the configured retention duration.
//
// Finish is idempotent: only the first call per Job has any effect, so callers
// can safely combine an explicit "happy path" call with a deferred safety-net
// Finish.
//
// If StartJob is called again on the same topic during the retention window,
// the new entry replaces the old one and is NOT removed by this Finish's
// retention timer (entries are compared by identity, not by topic).
func (j *Job) Finish(payload any) {
	j.entry.finishOnce.Do(func() { j.doFinish(payload) })
}

// Abort discards the entry without broadcasting a terminal or starting the
// retention window. Use it when the publisher knows the job will not produce
// a meaningful terminal event — e.g. a worker goroutine that failed to spawn,
// or an externally cancelled operation.
//
// Live subscribers are released because the entry's finished channel is
// closed; the live stream returns nil and clients will reconnect, where they
// will see ErrNoJob.
//
// Abort shares the once-guard with Finish: only the first of (Finish, Abort)
// per Job has any effect.
func (j *Job) Abort() {
	j.entry.finishOnce.Do(func() { j.doAbort() })
}

func (j *Job) doFinish(payload any) {
	t := j.tracker
	topic := j.topic
	e := j.entry

	t.mu.Lock()
	isCurrent := t.jobs[topic] == e // pointer identity: a newer StartJob may have installed a different entry
	t.mu.Unlock()

	if !isCurrent {
		// Stale entry: external serialization missed a beat and a newer StartJob
		// replaced this entry. Skip publish/retain/cleanup — those would corrupt
		// the replacement entry or schedule a duplicate cleanup. But still close
		// finished so subscribers stranded on this orphaned entry are released
		// rather than blocking on the channel forever.
		close(e.finished)
		t.debugf("jobtracker: Finish on stale entry for topic %q (subscribers released, no terminal retained)", topic)
		return
	}

	e.mu.Lock()
	e.terminal = payload
	e.terminalSet = true
	e.mu.Unlock()

	// Order matters: publish the terminal payload before closing finished, so
	// live subscribers see it as part of the normal event stream rather than
	// racing with the close signal.
	e.publish(payload)
	close(e.finished)

	t.debugf("jobtracker: finished job %q, retaining for %s", topic, t.retention)

	go t.scheduleCleanup(topic, e)
}

func (j *Job) doAbort() {
	t := j.tracker
	topic := j.topic
	e := j.entry

	t.mu.Lock()
	isCurrent := t.jobs[topic] == e
	if isCurrent {
		delete(t.jobs, topic)
	}
	t.mu.Unlock()

	// Always close finished, even on a stale entry, so subscribers attached to
	// the orphaned entry are released. The finishOnce guard in Job ensures this
	// runs at most once per Job handle.
	close(e.finished)

	if isCurrent {
		t.debugf("jobtracker: aborted job %q", topic)
	} else {
		t.debugf("jobtracker: Abort on stale entry for topic %q (subscribers released)", topic)
	}
}

// scheduleCleanup waits for the retention window or shutdown, whichever comes
// first, then removes the entry — but only if the entry still belongs to this
// Finish (a newer StartJob on the same topic during the window would have
// installed a different entry, owned by its own Finish).
func (t *Tracker) scheduleCleanup(topic string, e *jobEntry) {
	timer := time.NewTimer(t.retention)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-t.shutdown:
	}

	t.mu.Lock()
	c, ok := t.jobs[topic]
	if !ok || c != e {
		t.mu.Unlock()
		return
	}
	delete(t.jobs, topic)
	t.mu.Unlock()

	t.debugf("jobtracker: cleaned up entry for topic %q", topic)
}

// LookupJob returns the public state of the job for topic, or (nil, false) if
// no job has ever run for this topic, or the entry was already cleaned up after
// the retention window.
//
// Most callers should use Stream instead — this is exposed for tests and
// custom dispatch flows. Callers that read JobState.Terminal must type-assert
// it back to whatever the publisher passed to Job.Finish.
func (t *Tracker) LookupJob(topic string) (*JobState, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	e, ok := t.jobs[topic]
	if !ok {
		return nil, false
	}
	e.mu.RLock()
	terminal := e.terminal
	terminalSet := e.terminalSet
	e.mu.RUnlock()
	return &JobState{
		Terminal:    terminal,
		TerminalSet: terminalSet,
		Finished:    e.finished,
		entry:       e,
	}, true
}

// Stream is the high-level subscriber-side helper. Most SSE listen handlers
// should call this and not touch LookupJob directly.
//
// It dispatches the request based on the job's state:
//   - no entry          → returns ErrNoJob without writing anything (caller maps to 404)
//   - terminal retained → writes the terminal event and returns nil
//   - live job          → streams events until the job ends or the client disconnects
//
// Note: a client connecting at the exact moment Job.Finish runs may race past
// the terminal-replay branch (it sees TerminalSet == false) and then have its live
// subscription stopped immediately by the finished close; in that case the
// client receives no event. Standard SSE clients reconnect, and the second
// attempt takes the terminal-replay path.
func (t *Tracker) Stream(c echo.Context, topic string) error {
	state, ok := t.LookupJob(topic)
	if !ok {
		return ErrNoJob
	}
	if state.TerminalSet {
		// Best-effort write deadline so a slow or half-dead client cannot hang
		// the goroutine on either the SetHeaders flush or the WriteEvent write.
		// Real net.Conn writers support this; httptest.ResponseRecorder does
		// not — log and continue rather than refuse to replay. Set BEFORE
		// SetHeaders so the headers flush is also covered, not just WriteEvent.
		rc := http.NewResponseController(c.Response().Writer)
		if err := rc.SetWriteDeadline(time.Now().Add(t.heartbeat)); err != nil {
			t.debugf("jobtracker: set write deadline for terminal replay (continuing): %v", err)
		}
		sse.SetHeaders(c)
		// SetHeaders has already flushed, so the response is committed; bubbling
		// a write error back to echo would only produce a confused 500 after the
		// headers have shipped. Log and return nil — the contract here is "the
		// terminal was already retained, best-effort delivery to this client."
		if err := sse.WriteEvent(c, state.Terminal); err != nil {
			t.errorf("jobtracker: write retained terminal: %v", err)
		}
		return nil
	}
	return t.streamLive(c, state)
}

func (t *Tracker) streamLive(c echo.Context, state *JobState) error {
	ch := state.Subscribe()
	defer state.Unsubscribe(ch)

	// Mirror the terminal-replay branch: set a deadline before SetHeaders so a
	// half-dead client cannot hang the goroutine on the initial header flush
	// (the per-write deadlines below only kick in once the loop runs).
	rc := http.NewResponseController(c.Response().Writer)
	if err := rc.SetWriteDeadline(time.Now().Add(t.heartbeat)); err != nil {
		t.debugf("jobtracker: set write deadline for live SetHeaders (continuing): %v", err)
	}
	sse.SetHeaders(c)

	ticker := time.NewTicker(t.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case msg := <-ch:
			if !t.writeEvent(c, rc, msg) {
				return nil
			}
		case <-ticker.C:
			if !t.writeHeartbeat(c, rc) {
				return nil
			}
		case <-c.Request().Context().Done():
			t.debugf("jobtracker: client disconnected")
			return nil
		case <-t.shutdown:
			t.debugf("jobtracker: shutdown, closing live stream")
			return nil
		case <-state.Finished:
			// Drain any events buffered between the last loop iteration and
			// finished closing. Go's select picks randomly when multiple cases
			// are ready, so a Finish that publishes the terminal AND closes
			// finished can have the terminal lost without this drain.
			for {
				select {
				case msg := <-ch:
					if !t.writeEvent(c, rc, msg) {
						return nil
					}
				default:
					return nil
				}
			}
		}
	}
}

func (t *Tracker) writeEvent(c echo.Context, rc *http.ResponseController, msg any) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		t.errorf("jobtracker: marshal event payload: %v", err)
		return true
	}
	if err := rc.SetWriteDeadline(time.Now().Add(t.heartbeat)); err != nil {
		t.errorf("jobtracker: set write deadline: %v", err)
		return false
	}
	if _, err := fmt.Fprintf(c.Response().Writer, "data: %s\n\n", data); err != nil {
		t.errorf("jobtracker: write event: %v", err)
		return false
	}
	c.Response().Flush()
	return true
}

func (t *Tracker) writeHeartbeat(c echo.Context, rc *http.ResponseController) bool {
	if err := rc.SetWriteDeadline(time.Now().Add(t.heartbeat)); err != nil {
		t.errorf("jobtracker: set heartbeat write deadline: %v", err)
		return false
	}
	if _, err := fmt.Fprintf(c.Response().Writer, ":\n\n"); err != nil {
		t.errorf("jobtracker: write heartbeat: %v", err)
		return false
	}
	c.Response().Flush()
	return true
}

func (t *Tracker) debugf(format string, args ...any) {
	if t.lc != nil {
		t.lc.Debugf(format, args...)
	}
}

func (t *Tracker) errorf(format string, args ...any) {
	if t.lc != nil {
		t.lc.Errorf(format, args...)
	}
}

// publish fans payload out to every current subscriber. Channels with no room
// drop the event rather than block — a slow client must not stall the publisher.
func (e *jobEntry) publish(payload any) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for ch := range e.subscribers {
		select {
		case ch <- payload:
		default:
		}
	}
}
