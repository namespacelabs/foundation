// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/resources"
)

func register_OpCaptureServerConfig() {
	execution.RegisterHandlerFunc(func(ctx context.Context, inv *schema.SerializedInvocation, capture *resources.OpCaptureServerConfig) (*execution.HandleResult, error) {
		return &execution.HandleResult{
			Outputs: []execution.Output{
				{InstanceID: capture.ResourceInstanceId, Message: capture.Server},
			},
		}, nil
	})
}
