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
