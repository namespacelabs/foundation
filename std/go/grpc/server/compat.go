// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package server

import (
	"context"

	"namespacelabs.dev/foundation/std/go/server"
)

// The methods here are kept for backwards compatibility, and should be
// removed soon.

type Grpc = server.ServerImpl

func ListenGRPC(ctx context.Context, registerServices func(*Grpc)) error {
	return server.Listen(ctx, func(s server.Server) {
		registerServices(s.(*server.ServerImpl))
	})
}
