// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/schema"
	"tailscale.com/util/multierr"
)

type Handlers struct {
	handlers      []*HandlerRoute
	invokeHandler InvokeFunc
}

type MatchingHandlers struct {
	hs *Handlers
	matches
}

type HandlerRoute struct {
	h StackHandler
	matches
}

type matches struct {
	matchEnv *schema.Environment
}

func NewHandlers() *Handlers {
	return &Handlers{}
}

func (hs *Handlers) Any() *MatchingHandlers {
	matches := matches{}
	m := &MatchingHandlers{hs: hs, matches: matches}
	return m
}

func (hs *Handlers) MatchEnv(env *schema.Environment) *MatchingHandlers {
	matches := matches{matchEnv: env}
	m := &MatchingHandlers{hs: hs, matches: matches}
	return m
}

func (mh *MatchingHandlers) HandleStack(h StackHandler) *HandlerRoute {
	r := &HandlerRoute{h: h}
	r.matches = mh.matches
	mh.hs.handlers = append(mh.hs.handlers, r)
	return r
}

type InvokeFunc func(context.Context, Request) (*protocol.InvokeResponse, error)

func (mh *MatchingHandlers) HandleInvoke(f InvokeFunc) *MatchingHandlers {
	mh.hs.invokeHandler = f
	return mh
}

type runHandlers struct {
	h *Handlers
}

func (m matches) match(env *schema.Environment) bool {
	if m.matchEnv == nil {
		return true
	}

	if m.matchEnv.Name != "" && env.GetName() != m.matchEnv.Name {
		return false
	}

	if m.matchEnv.Purpose != schema.Environment_PURPOSE_UNKNOWN && env.GetPurpose() != m.matchEnv.Purpose {
		return false
	}

	if m.matchEnv.Runtime != "" && env.GetRuntime() != m.matchEnv.Runtime {
		return false
	}

	return true
}

func (rh runHandlers) Apply(ctx context.Context, req StackRequest, out *ApplyOutput) error {
	var errs []error

	for _, r := range rh.h.handlers {
		if !r.matches.match(req.Env) {
			continue
		}

		if err := r.h.Apply(ctx, req, out); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
}

func (rh runHandlers) Delete(ctx context.Context, req StackRequest, out *DeleteOutput) error {
	var errs []error

	for _, r := range rh.h.handlers {
		if !r.matches.match(req.Env) {
			continue
		}

		if err := r.h.Delete(ctx, req, out); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
}

func (rh runHandlers) Invoke(ctx context.Context, req Request) (*protocol.InvokeResponse, error) {
	if rh.h.invokeHandler == nil {
		return nil, status.Error(codes.Unavailable, "invoke not supported")
	}

	return rh.h.invokeHandler(ctx, req)
}
