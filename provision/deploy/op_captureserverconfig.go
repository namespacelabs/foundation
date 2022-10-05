// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/resources"
)

func register_OpCaptureServerConfig() {
	ops.RegisterHandlerFunc(func(ctx context.Context, inv *schema.SerializedInvocation, capture *resources.OpCaptureServerConfig) (*ops.HandleResult, error) {
		return &ops.HandleResult{
			Outputs: []ops.Output{
				{InstanceID: capture.ResourceInstanceId, Message: capture.Server},
			},
		}, nil
	})
}
