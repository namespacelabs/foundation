// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/engine/ops"
	internalres "namespacelabs.dev/foundation/internal/resources"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func Register_OpWaitForProviderResults() {
	ops.RegisterHandlerFunc(func(ctx context.Context, inv *schema.SerializedInvocation, wait *internalres.OpWaitForProviderResults) (*ops.HandleResult, error) {
		cluster, err := ops.Get(ctx, runtime.ClusterNamespaceInjection)
		if err != nil {
			return nil, err
		}

		if _, err := cluster.WaitForTermination(ctx, wait.Deployable); err != nil {
			return nil, err
		}

		return nil, nil
	})
}
