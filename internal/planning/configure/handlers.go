// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/schema"
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

func (hs *Handlers) Handler() AllHandlers {
	return handlersHandler{hs}
}

func (hs *Handlers) ServiceHandler() protocol.InvocationServiceServer {
	return protocolHandler{Handlers: hs.Handler()}
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

type handlersHandler struct {
	Handlers *Handlers
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

func (rh handlersHandler) Apply(ctx context.Context, req StackRequest, out *ApplyOutput) error {
	var errs []error

	for _, r := range rh.Handlers.handlers {
		if !r.matches.match(req.Env) {
			continue
		}

		if err := r.h.Apply(ctx, req, out); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
}

func (rh handlersHandler) Delete(ctx context.Context, req StackRequest, out *DeleteOutput) error {
	var errs []error

	for _, r := range rh.Handlers.handlers {
		if !r.matches.match(req.Env) {
			continue
		}

		if err := r.h.Delete(ctx, req, out); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
}

func (rh handlersHandler) Invoke(ctx context.Context, req Request) (*protocol.InvokeResponse, error) {
	if rh.Handlers.invokeHandler == nil {
		return nil, status.Error(codes.Unavailable, "invoke not supported")
	}

	return rh.Handlers.invokeHandler(ctx, req)
}

type protocolHandler struct {
	Handlers AllHandlers
}

func (i protocolHandler) Invoke(ctx context.Context, req *protocol.ToolRequest) (*protocol.ToolResponse, error) {
	return handleRequest(ctx, req, i.Handlers)
}
