//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package callstate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The whole contract. The premise the package rests on is the last two rows:
// ctx.Err() is context.DeadlineExceeded for both, so only the cause separates
// them. Both spellings this replaced got one of the two wrong.
func TestAbandoned(t *testing.T) {
	expired := func(parent context.Context) context.Context {
		ctx, cancel := context.WithDeadline(parent, time.Now().Add(-time.Second))
		t.Cleanup(cancel)
		return ctx
	}
	ceiling := func(d time.Duration) context.Context {
		ctx, cancel := context.WithTimeoutCause(context.Background(), d, ErrCeilingExpired)
		t.Cleanup(cancel)
		return ctx
	}
	cancelled := func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}

	for name, tc := range map[string]struct {
		ctx  context.Context
		want bool
	}{
		"a live context has not ended":                     {context.Background(), false},
		"our ceiling stamped its own cause":                {ceiling(-time.Second), false},
		"the caller cancelled":                             {cancelled(), true},
		"the caller's own deadline expired":                {expired(context.Background()), true},
		"a caller deadline under a live ceiling is theirs": {expired(ceiling(time.Hour)), true},
		"a ceiling that expired first owns the ending":     {expired(ceiling(-time.Second)), false},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, Abandoned(tc.ctx))
		})
	}

	require.Equal(t, ceiling(-time.Second).Err(), expired(context.Background()).Err(),
		"the premise: both expiries are context.DeadlineExceeded, so ctx.Err() cannot decide")
}
