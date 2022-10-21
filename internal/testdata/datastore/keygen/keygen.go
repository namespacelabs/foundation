// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"

	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/go-ids"
)

func main() {
	provisioning.HandleInvoke(func(ctx context.Context, r provisioning.Request) (*protocol.InvokeResponse, error) {
		return &protocol.InvokeResponse{
			Resource: &types.Resource{
				Contents: []byte(ids.NewRandomBase32ID(128)),
			},
		}, nil
	})
}
