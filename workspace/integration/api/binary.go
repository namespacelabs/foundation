// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package api

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var (
	// Key: typeUrl
	registeredBinaryIntegrations = map[string]func(context.Context, pkggraph.PackageLoader, pkggraph.Location, proto.Message) (*schema.Binary, error){}
)

// Must be called before ApplyIntegration.
func RegisterBinaryIntegration[V proto.Message](handler func(context.Context, pkggraph.PackageLoader, pkggraph.Location, V) (*schema.Binary, error)) {
	registeredBinaryIntegrations[protos.TypeUrl[V]()] = func(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location, data proto.Message) (*schema.Binary, error) {
		return handler(ctx, pl, loc, data.(V))
	}
}

func GenerateBinary(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location, binaryName string, data proto.Message) (*schema.Binary, error) {
	url := protos.TypeUrlForInstance(data)
	if i, ok := registeredBinaryIntegrations[url]; ok {
		binary, err := i(ctx, pl, loc, data)
		if err != nil {
			return nil, err
		}
		binary.Name = binaryName

		return binary, nil
	} else {
		return nil, fnerrors.UserError(loc, "unknown binary kind: %s", url)
	}
}

func GenerateBinaryAndAddToPackage(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, binaryName string, data proto.Message) (*schema.PackageRef, error) {
	binary, err := GenerateBinary(ctx, pl, pkg.Location, binaryName, data)
	if err != nil {
		return nil, err
	}

	pkg.Binaries = append(pkg.Binaries, binary)

	return schema.MakePackageRef(pkg.Location.PackageName, binaryName), nil
}
