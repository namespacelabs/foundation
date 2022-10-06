// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
)

func registerDelete() {
	execution.RegisterFuncs(execution.Funcs[*kubedef.OpDelete]{
		Handle: func(ctx context.Context, d *schema.SerializedInvocation, delete *kubedef.OpDelete) (*execution.HandleResult, error) {
			return nil, fnerrors.InternalError("unimplemented")
		},

		PlanOrder: func(_ *kubedef.OpDelete) (*schema.ScheduleOrder, error) {
			// XXX TODO
			return nil, nil
		},
	})
}
