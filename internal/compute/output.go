// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"

	"namespacelabs.dev/foundation/schema"
)

type Digester interface {
	ComputeDigest(context.Context, any) (schema.Digest, error)
}

type Output struct {
	NonDeterministic bool
	NotCacheable     bool
}

func (o Output) CanCache() bool {
	return !(o.NonDeterministic || o.NotCacheable)
}

func (o Output) DontCache() Output {
	return Output{
		NonDeterministic: o.NonDeterministic,
		NotCacheable:     true,
	}
}
