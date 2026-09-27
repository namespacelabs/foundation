// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package shared

import (
	"context"
	"strings"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	grpcprotos "namespacelabs.dev/foundation/std/grpc/protos"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// The standard grpc extension requires special handling as the provided type is
// is a usage-specific gRPC client class.
// TODO: make private once Go is fully migrated to the "shared" API.
func IsStdGrpcExtension(pkgName string, providerName string) bool {
	return pkgName == "namespacelabs.dev/foundation/std/grpc" && providerName == "Backend"
}

// TODO: make private once Go is fully migrate to the "shared" API.
func PrepareGrpcBackendDep(ctx context.Context, loader pkggraph.PackageLoader, dep *schema.Instantiate) (*ProtoTypeData, error) {
	backend := &grpcprotos.Backend{}
	if err := proto.Unmarshal(dep.Constructor.Value, backend); err != nil {
		return nil, err
	}

	pkg, err := loader.LoadByName(ctx, schema.PackageName(backend.PackageName))
	if err != nil {
		return nil, err
	}

	if pkg.Node().GetKind() != schema.Node_SERVICE {
		return nil, fnerrors.Newf("%s: must be a service", backend.PackageName)
	}

	// Finding the exported service. If no service name is provided, pick the first and only service.
	var exportedService *schema.GrpcExportService
	for _, svc := range pkg.Node().ExportService {
		if backend.ServiceName == "" || matchesService(svc.ProtoTypename, backend.ServiceName) {
			if exportedService != nil {
				return nil, fnerrors.Newf("%s: matching too many services, already had %s, got %s as well",
					backend.PackageName, exportedService.ProtoTypename, svc.ProtoTypename)
			}
			exportedService = svc
		}
	}

	if exportedService == nil {
		return nil, fnerrors.Newf("%s: no such service %q", backend.PackageName, backend.ServiceName)
	}

	return &ProtoTypeData{
		Name:           simpleServiceName(exportedService.ProtoTypename),
		SourceFileName: exportedService.GetProto()[0],
		Location:       pkg.Location,
		Kind:           ProtoService,
	}, nil
}

func matchesService(exported, provided string) bool {
	// Exported is always fully qualified, and provided may be a simple name.
	if exported == provided {
		return true
	}
	return simpleServiceName(exported) == provided
}

func simpleServiceName(typename string) string {
	parts := strings.Split(typename, ".")
	return parts[len(parts)-1]
}
