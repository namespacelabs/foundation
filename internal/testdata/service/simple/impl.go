// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package simple

import (
	"context"

	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
)

type Service struct {
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	proto.RegisterEmptyServiceServer(srv, &Service{})
}
