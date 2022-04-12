// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package data

import (
	"context"
)

func ProvideData(ctx context.Context, _ *Input) (*Data, error) {
	return &Data{}, nil
}
