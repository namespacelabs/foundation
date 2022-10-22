// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package data

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
)

func ProvideData(ctx context.Context, _ *Input) (*Data, error) {
	core.Log.Printf("[debug] path: %s", core.InstantiationPathFromContext(ctx))
	return &Data{}, nil
}
