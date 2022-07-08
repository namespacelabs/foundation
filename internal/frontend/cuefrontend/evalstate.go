// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"

	"namespacelabs.dev/foundation/internal/frontend/fncue"
)

type EvalFuncs struct {
	fetchers map[string]FetcherFunc
}

type Fetcher interface {
	Fetch(context.Context, *fncue.CueV, fncue.KeyAndPath) (interface{}, error)
}

var (
	consumeNoValue = "consume no value"
	ConsumeNoValue = &consumeNoValue
)

type FetcherFunc func(context.Context, *fncue.CueV) (interface{}, error)

func newFuncs() *EvalFuncs {
	return &EvalFuncs{
		fetchers: map[string]FetcherFunc{},
	}
}

func (s *EvalFuncs) copy() *EvalFuncs {
	n := &EvalFuncs{
		fetchers: map[string]FetcherFunc{},
	}

	for k, v := range s.fetchers {
		n.fetchers[k] = v
	}

	return n
}

func (s *EvalFuncs) WithFetcher(key string, f FetcherFunc) *EvalFuncs {
	n := s.copy()
	n.fetchers[key] = f
	return n
}

func (s *EvalFuncs) Fetch(ctx context.Context, v *fncue.CueV, kp fncue.KeyAndPath) (interface{}, error) {
	f := s.fetchers[kp.Key]
	if f != nil {
		return f(ctx, v)
	}
	return nil, nil
}
