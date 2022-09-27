// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package api

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

var (
	// Key: typeUrl
	registeredIntegrations = map[string]func(context.Context, *anypb.Any, *pkggraph.Package) error{}
)

// Must be called before ApplyIntegration.
func Register[V proto.Message](handler func(context.Context, V, *pkggraph.Package) error) {
	registeredIntegrations[protos.TypeUrl[V]()] = func(ctx context.Context, data *anypb.Any, pkg *pkggraph.Package) error {
		msg := protos.NewFromType[V]()
		if err := data.UnmarshalTo(msg); err != nil {
			return err
		}

		return handler(ctx, msg, pkg)
	}
}

func ApplyIntegration(ctx context.Context, pkg *pkggraph.Package) error {
	if pkg.Integration == nil {
		return nil
	}

	if i, ok := registeredIntegrations[pkg.Integration.Data.TypeUrl]; ok {
		return i(ctx, pkg.Integration.Data, pkg)
	} else {
		return fnerrors.UserError(pkg.Location, "unknown integration kind: %s", pkg.Integration)
	}
}
