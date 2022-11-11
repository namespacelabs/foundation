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
	// Key: typeUrl of ServerData
	serverApplyHandlers = map[string]func(context.Context, *schema.Environment, pkggraph.PackageLoader, *pkggraph.Package, *anypb.Any) error{}
	// Key: typeUrl of ServerData
	testApplyHandlers = map[string]func(context.Context, *schema.Environment, pkggraph.PackageLoader, *pkggraph.Package, *schema.Test, *anypb.Any) error{}
	// Key: typeUrl of BuildData
	createBinaryHandlers = map[string]func(context.Context, *schema.Environment, pkggraph.PackageLoader, pkggraph.Location, proto.Message) (*schema.Binary, error){}
)

func RegisterIntegration[ServerData proto.Message, BuildData proto.Message](impl Integration[ServerData, BuildData]) {
	registerServerApplyHandler(impl.ApplyToServer)
	registerTestApplyHandler(impl.ApplyToTest)
	registerCreateBinaryHandler(impl.CreateBinary)
}

func GenerateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, binaryName string, data proto.Message) (*schema.Binary, error) {
	url := protos.TypeUrlForInstance(data)
	if handler, ok := createBinaryHandlers[url]; ok {
		binary, err := handler(ctx, env, pl, loc, data)
		if err != nil {
			return nil, err
		}
		binary.Name = binaryName

		return binary, nil
	} else {
		return nil, fnerrors.NewWithLocation(loc, "unknown binary kind: %s", url)
	}
}

func ApplyServerIntegration(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package) error {
	if pkg.Integration == nil {
		return nil
	}

	if i, ok := serverApplyHandlers[pkg.Integration.Data.TypeUrl]; ok {
		return i(ctx, env, pl, pkg, pkg.Integration.Data)
	} else {
		return fnerrors.NewWithLocation(pkg.Location, "unknown integration kind: %s", pkg.Integration)
	}
}

func ApplyTestIntegration(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, test *schema.Test) error {
	if test.Integration == nil {
		return nil
	}

	if i, ok := testApplyHandlers[test.Integration.Data.TypeUrl]; ok {
		return i(ctx, env, pl, pkg, test, test.Integration.Data)
	} else {
		return fnerrors.NewWithLocation(pkg.Location, "unknown integration kind: %s", test.Integration)
	}
}

func registerServerApplyHandler[V proto.Message](handler func(context.Context, *schema.Environment, pkggraph.PackageLoader, *pkggraph.Package, V) error) {
	serverApplyHandlers[protos.TypeUrl[V]()] = func(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, data *anypb.Any) error {
		msg := protos.NewFromType[V]()
		if err := data.UnmarshalTo(msg); err != nil {
			return err
		}

		return handler(ctx, env, pl, pkg, msg)
	}
}

func registerTestApplyHandler[V proto.Message](handler func(context.Context, *schema.Environment, pkggraph.PackageLoader, *pkggraph.Package, *schema.Test, V) error) {
	testApplyHandlers[protos.TypeUrl[V]()] = func(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, test *schema.Test, data *anypb.Any) error {
		msg := protos.NewFromType[V]()
		if err := data.UnmarshalTo(msg); err != nil {
			return err
		}

		return handler(ctx, env, pl, pkg, test, msg)
	}
}

func registerCreateBinaryHandler[V proto.Message](handler func(context.Context, *schema.Environment, pkggraph.PackageLoader, pkggraph.Location, V) (*schema.Binary, error)) {
	// Here were already get a proto instance, no need to parse it from Any.
	createBinaryHandlers[protos.TypeUrl[V]()] = func(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data proto.Message) (*schema.Binary, error) {
		return handler(ctx, env, pl, loc, data.(V))
	}
}
