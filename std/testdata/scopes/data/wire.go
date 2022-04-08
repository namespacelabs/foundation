// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package data

import "context"

func ProvideData(_ context.Context, caller string, _ *Input) (*Data, error) {
	return &Data{Caller: []string{caller}}, nil
}
