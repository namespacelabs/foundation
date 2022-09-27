// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package api

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type IntegrationApplier interface {
	// For example, "namespace.so/from-dockerfile"
	Kind() string

	// Mutates pkg
	Apply(ctx context.Context, data *anypb.Any, pkg *pkggraph.Package) error
}
