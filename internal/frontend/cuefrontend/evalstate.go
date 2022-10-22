// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
)

type EvalFuncs struct {
	fetchers map[string]FetcherFunc
}

type Fetcher interface {
	Fetch(context.Context, cue.Value, fncue.KeyAndPath) (interface{}, error)
}

var (
	consumeNoValue = "consume no value"
	ConsumeNoValue = &consumeNoValue
)

type FetcherFunc func(context.Context, cue.Value) (interface{}, error)

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

func (s *EvalFuncs) Fetch(ctx context.Context, v cue.Value, kp fncue.KeyAndPath) (interface{}, error) {
	f := s.fetchers[kp.Key]
	if f != nil {
		return f(ctx, v)
	}
	return nil, nil
}
