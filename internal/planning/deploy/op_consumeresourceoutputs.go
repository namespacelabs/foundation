// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/internal/resources"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
)

// register_OpConsumeResourceOutputs registers a no-op handler that consumes
// resource instance outputs. The outputs are declared via RequiredOutput on the
// invocation, so the handler itself does nothing; it only exists to balance the
// plan's output accounting in provision-only deployments.
func register_OpConsumeResourceOutputs() {
	execution.RegisterHandlerFunc(func(ctx context.Context, inv *schema.SerializedInvocation, op *resources.OpConsumeResourceOutputs) (*execution.HandleResult, error) {
		return nil, nil
	})
}
