// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"strings"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type phase2plan struct {
	partial *fncue.Partial
	Value   *fncue.CueV
	Left    []fncue.KeyAndPath // injected values left to be filled.
}

type cueStartupPlan struct {
	Args *ArgsListOrMap    `json:"args"`
	Env  map[string]string `json:"env"`
}

var _ frontend.PreStartup = phase2plan{}

func (s phase2plan) EvalStartup(ctx context.Context, env ops.Environment, info frontend.StartupInputs, allocs []frontend.ValueWithPath) (*schema.StartupPlan, error) {
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

		plan.Env = raw.Env
		plan.Args = raw.Args.Parsed()
	}

	return plan, nil
}

func (s phase2plan) evalStartupStage(ctx context.Context, env ops.Environment, info frontend.StartupInputs) (*fncue.CueV, []fncue.KeyAndPath, error) {
	wenv, ok := env.(workspace.WorkspaceEnvironment)
	if !ok {
		return nil, nil, fnerrors.InternalError("expected a WorkspaceEnvironment")
	}

	inputs := newFuncs().
		WithFetcher(fncue.ServerDepIKw, FetchServer(wenv, info.Stack)).
		WithFetcher(fncue.FocusServerIKw, FetchFocusServer(info.ServerImage, info.Server)).
		WithFetcher(fncue.VCSIKw, FetchVCS(info.ServerRootAbs))

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
