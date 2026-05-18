//
// Copyright (C) 2026 IOTech Ltd
//

package progress

import (
	"errors"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/sse"
)

// ErrNoTopic — no active or recently-retained Job for the topic. Callers
// typically map to HTTP 404.
var ErrNoTopic = errors.New("progress: no active or recent job for topic")

// Subscribe attaches an SSE subscriber to topic.
//
//   - Running: write the latest cached event (if any), then live tail
//     until terminal / client disconnect / Tracker shutdown.
//   - Retained: write the cached terminal and close.
//   - Absent: ErrNoTopic.
//
// Late subscribers see only the most recent payload — callers must
// design events as monotonic snapshots (see package README).
func (t *Tracker) Subscribe(c echo.Context, topic string) error {
	// Hold t.mu.RLock across lookup + attach so the (j, subscription)
	// pair stays consistent — without it, a concurrent Start replacing a
	// retained entry would have us attach to a stale Job. Lock order:
	// Tracker.mu → Job.mu.
	t.mu.RLock()
	j, ok := t.jobs[topic]
	if !ok {
		t.mu.RUnlock()
		return ErrNoTopic
	}
	if hook := t.attachHookForTest; hook != nil {
		hook()
	}
	ch, replay, hasReplay, isRetained := t.attach(j)
	t.mu.RUnlock()

	if err := sse.WriteSSEHeaders(c, t.heartbeat, t.lc); err != nil {
		if !isRetained {
			t.unsubscribe(j, ch)
		}
		return err
	}

	if hasReplay {
		if err := sse.WriteSSEEvent(c, replay, t.heartbeat, t.lc); err != nil {
			t.lc.Errorf("sse: failed to write replay event: %v", err)
			if !isRetained {
				t.unsubscribe(j, ch)
			}
			return nil
		}
	}

	if isRetained {
		return nil
	}

	return t.runLiveLoop(c, j, ch)
}

// attach snapshots j.last and (if not terminal) registers a fresh fan-out
// channel under j.mu. The atomic snapshot+register prevents a concurrent
// Publish from being missed: any Publish after this point reaches the
// channel via latest-wins fan-out.
func (t *Tracker) attach(j *Job) (ch chan any, replay any, hasReplay bool, isRetained bool) {
	j.mu.Lock()
	defer j.mu.Unlock()

	replay = j.last
	hasReplay = j.hasLast

	if j.done {
		return nil, replay, hasReplay, true
	}

	ch = make(chan any, subscriberChanBuffer)
	j.subscribers[ch] = struct{}{}
	return ch, replay, hasReplay, false
}

// unsubscribe is idempotent against Finish, which also removes + closes.
func (t *Tracker) unsubscribe(j *Job, ch chan any) {
	j.mu.Lock()
	if _, ok := j.subscribers[ch]; ok {
		delete(j.subscribers, ch)
		close(ch)
	}
	j.mu.Unlock()
}

func (t *Tracker) runLiveLoop(c echo.Context, j *Job, ch chan any) error {
	defer t.unsubscribe(j, ch)

	heartbeat := time.NewTicker(t.heartbeat)
	defer heartbeat.Stop()

	reqCtx := c.Request().Context()
	for {
		select {
		case payload, open := <-ch:
			if !open {
				// Finish closed the channel as a wake-up signal; the
				// terminal payload lives in j.last (Finish writes it
				// under j.mu before closing). Even if the buffer carried
				// a pending snapshot, the terminal is separately read
				// from j.last after the close signal, so it is never at
				// the mercy of fan-out drops.
				j.mu.Lock()
				var terminal any
				hasTerminal := j.done && j.hasLast
				if hasTerminal {
					terminal = j.last
				}
				j.mu.Unlock()
				if hasTerminal {
					if err := sse.WriteSSEEvent(c, terminal, t.heartbeat, t.lc); err != nil {
						t.lc.Errorf("sse: failed to write terminal event: %v", err)
					}
				}
				return nil
			}
			if err := sse.WriteSSEEvent(c, payload, t.heartbeat, t.lc); err != nil {
				t.lc.Errorf("sse: failed to write event: %v", err)
				return nil
			}

		case <-heartbeat.C:
			if err := sse.WriteHeartbeat(c, t.heartbeat, t.lc); err != nil {
				t.lc.Warnf("sse: heartbeat write failed: %v", err)
				return nil
			}

		case <-reqCtx.Done():
			return nil
		case <-t.ctx.Done():
			return nil
		}
	}
}
