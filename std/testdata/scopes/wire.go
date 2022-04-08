// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package scopes

import "context"

func ProvideScopedData(_ context.Context, _ string, _ *Input, deps *ScopedDataDeps) (*ScopedData, error) {
	return &ScopedData{Data: deps.Data}, nil
}
