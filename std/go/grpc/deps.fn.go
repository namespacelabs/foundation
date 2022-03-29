// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This file was automatically generated.
package grpc

import (
	"context"

	_ "namespacelabs.dev/foundation/std/go/grpc/metrics"
	_ "namespacelabs.dev/foundation/std/monitoring/tracing"

	"google.golang.org/grpc"
)

type _checkProvideConn func(context.Context, string, *Conn) (*grpc.ClientConn, error)

var _ _checkProvideConn = ProvideConn