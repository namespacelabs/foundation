// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package data

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
)

func ProvideData(_ context.Context, caller fninit.Caller, _ *Input) (*Data, error) {
	return &Data{Caller: caller.String()}, nil
}
