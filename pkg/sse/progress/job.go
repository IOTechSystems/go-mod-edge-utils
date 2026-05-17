//
// Copyright (C) 2026 IOTech Ltd
//

package progress

import "sync"

// Job is the publisher handle for one topic-bound async operation. Owned
// by the goroutine that observed isNew=true from Start / StartOrJoin;
// subscribers reach it only via Tracker.Subscribe.
type Job struct {
	mu sync.Mutex

	done bool

	// last is the most recent payload (progress event or terminal). Late
	// subscribers replay only this single value, so callers must design
	// events as monotonic snapshots — see package README "Scope".
	last    any
	hasLast bool

	// subscribers receive fan-out via Publish. Each channel is size-1
	// with latest-wins semantics (Publish drains any pending stale
	// payload before pushing the new one). Finish closes (does not send
	// to) the channels; the terminal payload lives in j.last.
	subscribers map[chan any]struct{}

	// finished is closed by Finish; the cleanup goroutine watches it.
	finished chan struct{}
}

func newJob() *Job {
	return &Job{
		subscribers: make(map[chan any]struct{}),
		finished:    make(chan struct{}),
	}
}

// Publish updates the cached last payload and fans it out to subscribers
// with latest-wins semantics. A no-op once terminal. A slow subscriber
// that has not drained its previous payload sees that payload replaced
// by this newer one — for monotonic snapshot events, the most recent
// state is the only state that matters.
func (j *Job) Publish(data any) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.done {
		return
	}

	j.last = data
	j.hasLast = true

	for ch := range j.subscribers {
		// Latest-wins: drain any pending stale payload, then push fresh.
		// j.mu is held throughout, so the drain+push pair is atomic
		// relative to other Publishes on this Job.
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- data:
		default:
		}
	}
}

// Finish marks the job done, caches the terminal in j.last, and closes
// subscribers (signal only — they read the terminal from j.last under
// j.mu). Idempotent: defer job.Finish(failed) as a safety-net coexists
// with an explicit happy-path Finish.
//
// Success vs failure is encoded in the payload, not in two methods.
func (j *Job) Finish(data any) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.done {
		return
	}

	j.done = true
	j.last = data
	j.hasLast = true

	for ch := range j.subscribers {
		close(ch)
		delete(j.subscribers, ch)
	}

	close(j.finished)
}

// isTerminal acquires j.mu — callers must not already hold it. Used by
// Tracker.Start while holding Tracker.mu; the lock order Tracker.mu →
// Job.mu makes this safe.
func (j *Job) isTerminal() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.done
}
