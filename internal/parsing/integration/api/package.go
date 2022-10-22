// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var (
	// Key: typeUrl
	registeredPackageIntegrations = map[string]func(context.Context, *schema.Environment, pkggraph.PackageLoader, *anypb.Any, *pkggraph.Package) error{}
)

// Must be called before ApplyIntegration.
func RegisterPackageIntegration[V proto.Message](handler func(context.Context, *schema.Environment, pkggraph.PackageLoader, V, *pkggraph.Package) error) {
	registeredPackageIntegrations[protos.TypeUrl[V]()] = func(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, data *anypb.Any, pkg *pkggraph.Package) error {
		msg := protos.NewFromType[V]()
		if err := data.UnmarshalTo(msg); err != nil {
			return err
		}

		return handler(ctx, env, pl, msg, pkg)
	}
}

func ApplyPackageIntegration(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package) error {
	if pkg.Integration == nil {
		return nil
	}

	if i, ok := registeredPackageIntegrations[pkg.Integration.Data.TypeUrl]; ok {
		return i(ctx, env, pl, pkg.Integration.Data, pkg)
	} else {
		return fnerrors.UserError(pkg.Location, "unknown integration kind: %s", pkg.Integration)
	}
}
