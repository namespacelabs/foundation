// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"strings"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/args"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type phase2plan struct {
	owner   schema.PackageName
	partial *fncue.Partial
	Value   *fncue.CueV
	Left    []fncue.KeyAndPath // injected values left to be filled.
}

type cueStartupPlan struct {
	Args *args.ArgsListOrMap `json:"args"`
	Env  *args.EnvMap        `json:"env"`
}

var _ pkggraph.PreStartup = phase2plan{}

func (s phase2plan) EvalStartup(ctx context.Context, env pkggraph.Context, info pkggraph.StartupInputs, allocs []pkggraph.ValueWithPath) (*schema.StartupPlan, error) {
	plan := &schema.StartupPlan{}

	res, err := fncue.SerializedEval(s.partial, func() (*fncue.CueV, error) {
		res, _, err := s.evalStartupStage(ctx, env, info)
		if err != nil {
			return nil, err
		}

		for _, alloc := range allocs {
			res = res.FillPath(cue.ParsePath(alloc.Need.CuePath), alloc.Value)
		}

		return res, nil
	})

	if err != nil {
		return nil, err
	}

	if v := lookupTransition(res, "startup"); v.Exists() {
		if err := v.Val.Validate(cue.Concrete(true)); err != nil {
			return nil, err
		}

		var raw cueStartupPlan
		if err := v.Val.Decode(&raw); err != nil {
			return nil, err
		}

		envVar, err := raw.Env.Parsed(s.owner)
		if err != nil {
			return nil, err
		}

		plan.Env = envVar
		plan.Args = raw.Args.Parsed()
	}

	return plan, nil
}

func (s phase2plan) evalStartupStage(ctx context.Context, wenv pkggraph.Context, info pkggraph.StartupInputs) (*fncue.CueV, []fncue.KeyAndPath, error) {
	inputs := newFuncs().
		WithFetcher(fncue.ServerDepIKw, FetchServer(wenv, info.Stack)).
		WithFetcher(fncue.VCSIKw, func(ctx context.Context, v cue.Value) (interface{}, error) {
			return nil, fnerrors.BadInputError("inputs.#VCS is no longer supported, please update your foundation dependency")
		})

	vv, left, err := applyInputs(ctx, inputs, s.Value, s.Left)
	if err != nil {
		return nil, nil, err
	}

	if len(left) > 0 {
		var keys uniquestrings.List
		for _, kv := range left {
			keys.Add(kv.Key)
		}

		return nil, nil, fnerrors.InternalError("inputs not provisioned: %s", strings.Join(keys.Strings(), ", "))
	}

	return vv, left, err
}
