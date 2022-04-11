// This file was automatically generated.
package grpc

import (
	"context"

	"google.golang.org/grpc"
)

type _checkProvideConn func(context.Context, *Backend) (*grpc.ClientConn, error)

var _ _checkProvideConn = ProvideConn
