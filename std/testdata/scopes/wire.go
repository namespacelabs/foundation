// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package scopes

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
)

func ProvideScopedData(_ context.Context, _ fninit.Caller, _ *Input, deps *ScopedDataDeps) (*ScopedData, error) {
	return &ScopedData{Data: deps.Data}, nil
}
