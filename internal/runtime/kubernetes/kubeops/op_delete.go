// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeops

import (
	"context"

	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
)

func registerDelete() {
	execution.RegisterFuncs(execution.Funcs[*kubedef.OpDelete]{
		Handle: func(ctx context.Context, d *schema.SerializedInvocation, delete *kubedef.OpDelete) (*execution.HandleResult, error) {
			return nil, fnerrors.InternalError("unimplemented")
		},

		PlanOrder: func(ctx context.Context, _ *kubedef.OpDelete) (*schema.ScheduleOrder, error) {
			// XXX TODO
			return nil, nil
		},
	})
}
