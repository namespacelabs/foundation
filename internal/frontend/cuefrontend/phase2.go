// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"strings"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/uniquestrings"
)

type phase2plan struct {
	Value *fncue.CueV
	Left  []fncue.KeyAndPath // injected values left to be filled.
}

type cueStartupPlan struct {
	Args *argsListOrMap    `json:"args"`
	Env  map[string]string `json:"env"`
}

var _ frontend.PreStartup = phase2plan{}

func (s phase2plan) EvalStartup(ctx context.Context, info frontend.StartupInputs, allocs []frontend.ValueWithPath) (frontend.StartupPlan, error) {
	var plan frontend.StartupPlan

	res, _, err := s.evalStartupStage(ctx, info)
	if err != nil {
		return plan, err
	}

	for _, alloc := range allocs {
		res = res.FillPath(cue.ParsePath(alloc.Need.CuePath), alloc.Value)
	}

	if v := lookupTransition(res, "startup"); v.Exists() {
		if err := v.Val.Validate(cue.Concrete(true)); err != nil {
			return plan, err
		}

		var raw cueStartupPlan
		if err := v.Val.Decode(&raw); err != nil {
			return frontend.StartupPlan{}, err
		}

		plan.Env = raw.Env
		plan.Args = raw.Args.Parsed()
	}

	return plan, nil
}

func (s phase2plan) evalStartupStage(ctx context.Context, info frontend.StartupInputs) (*fncue.CueV, []fncue.KeyAndPath, error) {
	inputs := newFuncs().
		WithFetcher(fncue.ServerDepIKw, FetchServer(info.Stack)).
		WithFetcher(fncue.FocusServerIKw, FetchFocusServer(info.ServerImage, info.Server))

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
