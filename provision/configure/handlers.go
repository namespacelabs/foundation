// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"tailscale.com/util/multierr"
)

type Handlers struct{ handlers []*HandlerRoute }

type MatchingHandlers struct {
	hs *Handlers
	matches
}

type HandlerRoute struct {
	h Handler
	matches
}

type matches struct {
	matchEnv *schema.Environment
}

func NewRegistration() *Handlers {
	return &Handlers{}
}

func (hs *Handlers) MatchEnv(env *schema.Environment) *MatchingHandlers {
	matches := matches{matchEnv: env}
	m := &MatchingHandlers{hs: hs, matches: matches}
	return m
}

func (hs *Handlers) Handler(h Handler) *HandlerRoute {
	route := &HandlerRoute{h: h}
	hs.handlers = append(hs.handlers, route)
	return route
}

func (mh *MatchingHandlers) Handle(h Handler) *HandlerRoute {
	r := &HandlerRoute{h: h}
	r.matches = mh.matches
	mh.hs.handlers = append(mh.hs.handlers, r)
	return r
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

func (rh runHandlers) Apply(ctx context.Context, req Request, out *ApplyOutput) error {
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

func (rh runHandlers) Delete(ctx context.Context, req Request, out *DeleteOutput) error {
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
