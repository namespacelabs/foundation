// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/protocolbuffers/txtpbfmt/parser"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	grpcprotos "namespacelabs.dev/foundation/std/grpc/protos"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

// Prepare codegen data for a server.
func PrepareServerData(ctx context.Context, loader workspace.Packages, loc workspace.Location, srv *schema.Server, fmwk schema.Framework) (ServerData, error) {
	var serverData ServerData

	userImports := make(map[schema.PackageName]bool)
	for _, i := range srv.GetUserImports() {
		userImports[schema.Name(i)] = true
	}

	for _, ref := range srv.GetImportedPackages() {
		pkg, err := loader.LoadByName(ctx, ref)
		if err != nil {
			return serverData, err
		}

		if pkg.Node().GetKind() == schema.Node_SERVICE {
			serverData.Services = append(serverData.Services, EmbeddedServiceData{
				Location: pkg.Location,
				HasDeps:  len(pkg.Node().GetInstantiate()) > 0,
			})
		}

		node := pkg.Node()

		// Only adding initializers from direct dependencies that have codegen
		// for the given framework.
		if userImports[pkg.PackageName()] &&
			(node == nil || slices.Contains(node.CodegeneratedFrameworks(), fmwk)) {
			serverData.ImportedInitializers = append(serverData.ImportedInitializers, pkg.Location)
		}
	}
	serverData.ImportedInitializers = removeDuplicates(serverData.ImportedInitializers)

	return serverData, nil
}

func PrepareNodeData(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, fmwk schema.Framework) (NodeData, error) {
	nodeData := NodeData{
		Kind:        n.Kind,
		PackageName: n.PackageName,
	}

	if i := n.InitializerFor(fmwk); i != nil {
		nodeData.Initializer = &PackageInitializerData{
			InitializeBefore: i.InitializeBefore,
			InitializeAfter:  i.InitializeAfter,
		}
	}

	if len(n.Instantiate) > 0 {
		deps, err := prepareDeps(ctx, loader, fmwk, n.Instantiate)
		if err != nil {
			return NodeData{}, err
		}

		for _, d := range deps {
			nodeData.ImportedInitializers = append(nodeData.ImportedInitializers, d.ProviderLocation)
		}
		nodeData.ImportedInitializers = removeDuplicates(nodeData.ImportedInitializers)

		nodeData.Deps = deps
	}

	for _, p := range n.Provides {
		for _, a := range p.AvailableIn {
			if a.ProvidedInFrameworks()[fmwk] {
				scopeDeps, err := prepareDeps(ctx, loader, fmwk, p.Instantiate)
				if err != nil {
					return NodeData{}, err
				}

				nodeData.Providers = append(nodeData.Providers, ProviderData{
					Name:      p.Name,
					Location:  loc,
					InputType: convertProtoMessageType(p.Type, loc),
					ProviderType: ProviderTypeData{
						ParsedType:      a,
						IsParameterized: IsStdGrpcExtension(n.PackageName, p.Name),
					},
					ScopedDeps: scopeDeps,
				})
			}
		}
	}

	return nodeData, nil
}

// The standard grpc extension requires special handling as the provided type is
// is a usage-specific gRPC client class.
// TODO: make private once Go is fully migrate to the "shared" API.
func IsStdGrpcExtension(pkgName string, providerName string) bool {
	return pkgName == "namespacelabs.dev/foundation/std/grpc" && providerName == "Backend"
}

func prepareDeps(ctx context.Context, loader workspace.Packages, fmwk schema.Framework, instantiates []*schema.Instantiate) ([]DependencyData, error) {
	if len(instantiates) == 0 {
		return nil, nil
	}

	deps := []DependencyData{}
	for _, dep := range instantiates {
		depData, err := prepareDep(ctx, loader, fmwk, dep)
		if err != nil {
			return nil, err
		}

		if depData != nil {
			deps = append(deps, *depData)
		}
	}

	return deps, nil
}

// Returns nil if the provider is not available in the current framework.
func prepareDep(ctx context.Context, loader workspace.Packages, fmwk schema.Framework, dep *schema.Instantiate) (*DependencyData, error) {
	pkg, err := loader.LoadByName(ctx, schema.PackageName(dep.PackageName))
	if err != nil {
		return nil, fnerrors.UserError(nil, "failed to load %s/%s: %w", dep.PackageName, dep.Type, err)
	}

	_, p := workspace.FindProvider(pkg, schema.PackageName(dep.PackageName), dep.Type)
	if p == nil {
		return nil, fnerrors.UserError(nil, "didn't find a provider for %s/%s", dep.PackageName, dep.Type)
	}

	var provider *ProviderTypeData
	// Special case: generate the gRPC client definition.
	if IsStdGrpcExtension(dep.PackageName, dep.Type) {
		grpcClientType, err := PrepareGrpcBackendDep(ctx, loader, dep)
		if err != nil {
			return nil, err
		}
		provider = &ProviderTypeData{
			Type:            grpcClientType,
			IsParameterized: true,
		}
	} else {
		for _, prov := range p.AvailableIn {
			if prov.ProvidedInFrameworks()[fmwk] {
				if provider != nil {
					return nil, fnerrors.UserError(nil, "multiple providers available for %s/%s", dep.PackageName, dep.Type)
				}
				provider = &ProviderTypeData{
					ParsedType: prov,
				}
			}
		}
	}

	if provider == nil {
		// This provider is not available in the current framework.
		return nil, nil
	}

	providerInput, err := serializeProto(ctx, pkg, p, dep)
	if err != nil {
		return nil, err
	}

	return &DependencyData{
		Name:              dep.Name,
		ProviderName:      p.Name,
		ProviderInputType: convertProtoMessageType(p.Type, pkg.Location),
		ProviderType:      *provider,
		ProviderLocation:  pkg.Location,
		ProviderInput:     *providerInput,
	}, nil
}

// TODO: make private once Go is fully migrate to the "shared" API.
func PrepareGrpcBackendDep(ctx context.Context, loader workspace.Packages, dep *schema.Instantiate) (*ProtoTypeData, error) {
	backend := &grpcprotos.Backend{}
	if err := proto.Unmarshal(dep.Constructor.Value, backend); err != nil {
		return nil, err
	}

	pkg, err := loader.LoadByName(ctx, schema.PackageName(backend.PackageName))
	if err != nil {
		return nil, err
	}

	if pkg.Node().GetKind() != schema.Node_SERVICE {
		return nil, fnerrors.UserError(nil, "%s: must be a service", backend.PackageName)
	}

	// Finding the exported service. If no service name is provided, pick the first and only service.
	var exportedService *schema.GrpcExportService
	for _, svc := range pkg.Node().ExportService {
		if backend.ServiceName == "" || matchesService(svc.ProtoTypename, backend.ServiceName) {
			if exportedService != nil {
				return nil, fnerrors.UserError(nil, "%s: matching too many services, already had %s, got %s as well",
					backend.PackageName, exportedService.ProtoTypename, svc.ProtoTypename)
			}
			exportedService = svc
		}
	}

	if exportedService == nil {
		return nil, fnerrors.UserError(nil, "%s: no such service %q", backend.PackageName, backend.ServiceName)
	}

	return &ProtoTypeData{
		Name:           fmt.Sprintf("%sClient", simpleServiceName(exportedService.ProtoTypename)),
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

func convertProtoMessageType(t *schema.TypeDef, loc workspace.Location) ProtoTypeData {
	nameParts := strings.Split(string(t.Typename), ".")
	// TODO: check that the sources contain at least one file.
	return ProtoTypeData{
		Name:           nameParts[len(nameParts)-1],
		SourceFileName: t.Source[0],
		Location:       loc,
		Kind:           ProtoMessage,
	}
}

// Copied from "languages/golang/dependency.go#serializeProto"
func serializeProto(ctx context.Context, pkg *workspace.Package, provides *schema.Provides, instance *schema.Instantiate) (*SerializedProto, error) {
	serializedProto := SerializedProto{
		Comments: []string{},
	}

	parsed, ok := pkg.Provides[provides.Name]
	if !ok {
		return nil, fnerrors.InternalError("%s: protos were not loaded as expected?", instance.PackageName)
	}

	files, msgdesc, err := protos.LoadMessageByName(parsed, provides.Type.Typename)
	if err != nil {
		return nil, fnerrors.InternalError("%s: failed to load message %q: %w", instance.PackageName, provides.Type.Typename, err)
	}

	raw := dynamicpb.NewMessage(msgdesc)
	if err := proto.Unmarshal(instance.Constructor.Value, raw.Interface()); err != nil {
		return nil, fnerrors.InternalError("failed to unmarshal constructor: %w", err)
	}

	// Clean up all values which are not meant to be shipped into the binary.
	protos.CleanupForNonProvisioning(raw)

	deterministicBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(raw.Interface())
	if err != nil {
		return nil, fnerrors.InternalError("failed to marshal depvar: %w", err)
	}

	serializedProto.Content = deterministicBytes

	resolver, err := protos.AsResolver(files)
	if err != nil {
		return nil, fnerrors.InternalError("failed to create resolver: %w", err)
	}

	serialized, err := prototext.MarshalOptions{Multiline: true, Resolver: resolver}.Marshal(raw.Interface())
	if err == nil {
		stableFmt, err := parser.Format(serialized)
		if err == nil {
			serializedProto.Comments = strings.Split(string(stableFmt), "\n")
		}
	}

	return &serializedProto, nil
}

func removeDuplicates(list []workspace.Location) []workspace.Location {
	seen := make(map[schema.PackageName]bool)
	result := []workspace.Location{}

	for _, item := range list {
		if !seen[item.PackageName] {
			result = append(result, item)
			seen[item.PackageName] = true
		}
	}

	return result
}
