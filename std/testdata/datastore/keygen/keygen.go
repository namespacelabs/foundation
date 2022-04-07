// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"

	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/go-ids"
)

func main() {
	configure.HandleInvoke(func(ctx context.Context, r configure.Request) (*protocol.InvokeResponse, error) {
		return &protocol.InvokeResponse{
			RawOutput: []byte(ids.NewRandomBase32ID(128)),
		}, nil
	})
}
