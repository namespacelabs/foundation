// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package shared

import (
	"context"
	"strings"

	"github.com/protocolbuffers/txtpbfmt/parser"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

// Prepare codegen data for a server.
func PrepareServerData(ctx context.Context, loader workspace.Packages, loc workspace.Location, srv *schema.Server) (ServerData, error) {
	var serverData ServerData

	for _, ref := range srv.GetImportedPackages() {
		pkg, err := loader.LoadByName(ctx, ref)
		if err != nil {
			return serverData, err
		}

		if pkg.Node().GetKind() == schema.Node_SERVICE {
			serverData.Services = append(serverData.Services, EmbeddedServiceData{
				Location: pkg.Location,
			})
		}
	}

	return serverData, nil
}

func PrepareNodeData(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, fmwk schema.Framework) (NodeData, error) {
	var nodeData NodeData

	if len(n.Instantiate) > 0 {
		deps, err := prepareDeps(ctx, loader, fmwk, n.Instantiate)
		if err != nil {
			return NodeData{}, err
		}

		nodeData.SingletonDeps = deps
	}
	nodeData.HasService = n.ExportService != nil

	for _, p := range n.Provides {
		for _, a := range p.AvailableIn {
			if a.ProvidedInFrameworks()[fmwk] {
				scopeDeps, err := prepareDeps(ctx, loader, fmwk, p.Instantiate)
				if err != nil {
					return NodeData{}, err
				}

				nodeData.Providers = append(nodeData.Providers, ProviderData{
					Name:         p.Name,
					InputType:    convertType(p.Type, schema.PackageName(n.PackageName)),
					ProviderType: a,
					ScopedDeps:   scopeDeps,
				})
			}
		}
	}

	return nodeData, nil
}

func prepareDeps(ctx context.Context, loader workspace.Packages, fmwk schema.Framework, instantiates []*schema.Instantiate) ([]DependencyData, error) {
	deps := []DependencyData{}
	for _, dep := range instantiates {
		pkg, err := loader.LoadByName(ctx, schema.PackageName(dep.PackageName))
		if err != nil {
			return nil, fnerrors.UserError(nil, "failed to load %s/%s: %w", dep.PackageName, dep.Type, err)
		}

		_, p := workspace.FindProvider(pkg, schema.PackageName(dep.PackageName), dep.Type)
		if p == nil {
			return nil, fnerrors.UserError(nil, "didn't find a provider for %s/%s", dep.PackageName, dep.Type)
		}

		var provider *schema.Provides_AvailableIn
		for _, prov := range p.AvailableIn {
			if prov.ProvidedInFrameworks()[fmwk] {
				provider = prov
				break
			}
		}

		providerInput, err := serializeProto(ctx, pkg, p, dep)
		if err != nil {
			return nil, err
		}

		deps = append(deps, DependencyData{
			Name:                     dep.Name,
			ProviderName:             p.Name,
			ProviderInputType:        convertType(p.Type, schema.PackageName(dep.PackageName)),
			ProviderType:             provider,
			ProviderHasScopedDeps:    len(p.Instantiate) > 0,
			ProviderHasSingletonDeps: len(pkg.Node().Instantiate) > 0,
			ProviderLocation:         pkg.Location,
			ProviderInput:            *providerInput,
		})
	}
	return deps, nil
}

func convertType(t *schema.TypeDef, pkgName schema.PackageName) TypeData {
	nameParts := strings.Split(string(t.Typename), ".")
	// TODO(@nicolasalt): check that the sources contain at least one file.
	return TypeData{
		Name:           nameParts[len(nameParts)-1],
		SourceFileName: t.Source[0],
		PackageName:    pkgName,
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
