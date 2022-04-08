// This file was automatically generated.
package grpc

import (
	"context"

	"google.golang.org/grpc"
	fninit "namespacelabs.dev/foundation/std/go/core/init"
)

type _checkProvideConn func(context.Context, fninit.Caller, *Backend) (*grpc.ClientConn, error)

var _ _checkProvideConn = ProvideConn
