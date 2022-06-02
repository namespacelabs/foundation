// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/workspace/compute"
)

func ResolveResource(ctx context.Context, env ops.Environment, src *types.DeferredResource) (*types.Resource, error) {
	if src == nil {
		return nil, fnerrors.New("failed to retrieve value: no value")
	}

	if src.Inline != nil {
		return src.Inline, nil
	}

	invocation, err := tools.Invoke(ctx, env, src.FromInvocation)
	if err != nil {
		return nil, err
	}

	result, err := compute.GetValue(ctx, invocation)
	if err != nil {
		return nil, err
	}

	return result.GetResource(), nil
}
