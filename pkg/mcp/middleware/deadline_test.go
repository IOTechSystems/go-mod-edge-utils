//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// deadline_test.go — that the ceiling bounds the methods it is meant to, leaves
// the ones it is not alone, and says so when it cuts a call that may already have
// changed something upstream.

package middleware

import (
	"context"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/callstate"
)

// callFor builds a tools/call request for one tool name.
func callFor(name string) *sdkmcp.CallToolRequest {
	return &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Name: name}}
}

// Which methods get a ceiling, and which must not. The handler returns at once
// when it has no deadline rather than blocking, so a regression fails on the
// assertion instead of hanging the package.
func TestDeadline_BoundsTheCeilingedMethodsAndOnlyThose(t *testing.T) {
	for _, tc := range []struct {
		method    string
		req       sdkmcp.Request
		wantBound bool
	}{
		{methodToolsCall, callFor("any_tool"), true},
		{methodToolsList, &sdkmcp.ListToolsRequest{}, true},
		{"resources/read", &sdkmcp.ListToolsRequest{}, false},
	} {
		t.Run(tc.method, func(t *testing.T) {
			saw := make(chan error, 1)
			h := Deadline(50 * time.Millisecond)(
				func(ctx context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
					if _, ok := ctx.Deadline(); !ok {
						saw <- nil
						return nil, nil
					}
					<-ctx.Done()
					saw <- ctx.Err()
					return nil, ctx.Err()
				})

			_, _ = h(context.Background(), tc.method, tc.req)

			if tc.wantBound {
				assert.ErrorIs(t, <-saw, context.DeadlineExceeded,
					"the handler must see the ceiling, not merely be abandoned by the caller")
				return
			}
			assert.NoError(t, <-saw, "a ceiling here would cut the connection, not a call")
		})
	}
}

// ⚠ A cross-package property with its ends far apart: this stamps, and
// authz.Transport.RoundTrip reads. Dropping the Cause compiles, cuts calls exactly
// as before, and breaks the classification silently.
func TestDeadline_StampsItsOwnCauseSoTheOwnerOfTheDeadlineIsKnown(t *testing.T) {
	var seen context.Context
	h := Deadline(50 * time.Millisecond)(
		func(ctx context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
			seen = ctx
			<-ctx.Done()
			return nil, ctx.Err()
		})

	_, _ = h(context.Background(), methodToolsCall, callFor("any_tool"))

	require.ErrorIs(t, seen.Err(), context.DeadlineExceeded,
		"control: the ceiling must actually have fired")
	assert.ErrorIs(t, context.Cause(seen), callstate.ErrCeilingExpired,
		"the cause is what tells authz this deadline was OURS, not the caller's")

	// The other half: a deadline we did not set must NOT carry our cause, or the
	// stamp would say nothing.
	callerCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var underCaller context.Context
	_, _ = Deadline(time.Hour)(
		func(ctx context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
			underCaller = ctx
			<-ctx.Done()
			return nil, ctx.Err()
		})(callerCtx, methodToolsCall, callFor("any_tool"))

	assert.NotErrorIs(t, context.Cause(underCaller), callstate.ErrCeilingExpired,
		"the caller's own deadline is not ours to claim")
}

// Which duration ends up on the handler's context: the installed ceiling carries
// the constant, and a tighter caller deadline wins.
func TestDeadline_TheDeadlineOnTheHandlersContext(t *testing.T) {
	for name, tc := range map[string]struct {
		mw      sdkmcp.Middleware
		parent  func() (context.Context, context.CancelFunc)
		wantSec float64
	}{
		"Deadline carries ToolCallCeiling": {mw: Deadline(ToolCallCeiling), wantSec: ToolCallCeiling.Seconds()},
		"a tighter caller deadline wins": {
			mw: Deadline(time.Hour),
			parent: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 2*time.Second)
			},
			wantSec: 2,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.Background(), func() {}
			if tc.parent != nil {
				ctx, cancel = tc.parent()
			}
			defer cancel()

			var seen context.Context
			h := tc.mw(func(c context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
				seen = c
				return nil, nil
			})

			_, err := h(ctx, methodToolsCall, callFor("any_tool"))
			require.NoError(t, err)

			deadline, ok := seen.Deadline()
			require.True(t, ok)
			assert.InDelta(t, tc.wantSec, time.Until(deadline).Seconds(), 1)
		})
	}
}

// A non-positive ceiling hands every call an already-expired context, and from the
// outermost middleware that fails the WHOLE tool surface. Loud at boot, not silent
// at runtime.
func TestDeadline_RefusesANonPositiveCeiling(t *testing.T) {
	for _, ceiling := range []time.Duration{0, -time.Second} {
		assert.Panics(t, func() { Deadline(ceiling) }, "ceiling %s", ceiling)
	}
	assert.NotPanics(t, func() { Deadline(ToolCallCeiling) }, "control: a real ceiling must build")
}
