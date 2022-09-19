// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"

	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

func registerDeleteList() {
	ops.RegisterFuncs(ops.Funcs[*kubedef.OpDeleteList]{
		Handle: func(ctx context.Context, d *schema.SerializedInvocation, deleteList *kubedef.OpDeleteList) (*ops.HandleResult, error) {
			return nil, fnerrors.InternalError("unimplemented")
		},

		PlanOrder: func(_ *kubedef.OpDeleteList) (*schema.ScheduleOrder, error) {
			// XXX TODO
			return nil, nil
		},
	})
}
