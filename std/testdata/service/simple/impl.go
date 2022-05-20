// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package simple

import (
	"context"

	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/testdata/service/proto"
)

type Service struct {
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	proto.RegisterEmptyServiceServer(srv, &Service{})
}
