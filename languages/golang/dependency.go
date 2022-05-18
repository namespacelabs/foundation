// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"bytes"
	"context"
	"encoding/base64"

	"github.com/kr/text"
	"github.com/protocolbuffers/txtpbfmt/parser"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/gosupport"
	"namespacelabs.dev/foundation/languages/shared"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

type nodeLoc struct {
	Location workspace.Location
	Node     *schema.Node
}

type initializer struct {
	nodeLoc
	initializeBefore []string
	initializeAfter  []string
}

type instancedDepList struct {
	services     []nodeLoc
	instances    []*instancedDep
	initializers []initializer
}

type instancedDep struct {
	Location    workspace.Location
	Parent      *schema.Node
	Scope       *schema.Provides // Parent provider - nil for singleton dependencies.
	Instance    *schema.Instantiate
	Provisioned *typeProvider
}

type typeProvider struct {
	Provides *schema.Provides

	PackageName schema.PackageName
	GoPackage   string // GoPackage where the ProvisionYYY() method is defined.
	DepVars     []gosupport.TypeDef
	Method      string
	Args        []string

	SerializedMsg string
	ProtoComments string

	Dependencies []*typeProvider
}

func expandInstancedDeps(ctx context.Context, loader workspace.Packages, includes []schema.PackageName) (instancedDepList, error) {
	var e instancedDepList

	for _, ref := range includes {
		pkg, err := loader.LoadByName(ctx, ref)
		if err != nil {
			return e, err
		}

		referenced := pkg.Node()
		if referenced == nil {
			continue
		}

		if err := expandNode(ctx, loader, pkg.Location, referenced, true, &e); err != nil {
			return e, err
		}

		if referenced.GetKind() == schema.Node_SERVICE {
			e.services = append(e.services, nodeLoc{Location: pkg.Location, Node: referenced})
		}

		if x := referenced.InitializerFor(schema.Framework_GO_GRPC); x != nil {
			e.initializers = append(e.initializers, initializer{
				nodeLoc:          nodeLoc{Location: pkg.Location, Node: referenced},
				initializeBefore: x.InitializeBefore,
				initializeAfter:  x.InitializeAfter,
			})
		}
	}

	return e, nil
}

func expandNode(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, produceSerialized bool, e *instancedDepList) error {
	if !isGoNode(n) {
		return nil
	}

	for k, dep := range n.GetInstantiate() {
		var prov typeProvider

		if err := makeDep(ctx, loader, dep, produceSerialized, &prov); err != nil {
			return fnerrors.UserError(loc, "%s.dependency[%d]: %w", n.GetPackageName(), k, err)
		}

		if len(prov.DepVars) > 0 {
			e.instances = append(e.instances, &instancedDep{
				Location:    loc,
				Parent:      n,
				Instance:    dep,
				Provisioned: &prov,
			})
		}
	}

	for _, p := range n.Provides {
		for k, dep := range p.GetInstantiate() {
			var prov typeProvider

			if err := makeDep(ctx, loader, dep, produceSerialized, &prov); err != nil {
				return fnerrors.UserError(loc, "%s.%s.dependency[%d]: %w", n.GetPackageName(), p.Name, k, err)
			}

			if len(prov.DepVars) > 0 {
				e.instances = append(e.instances, &instancedDep{
					Location:    loc,
					Parent:      n,
					Instance:    dep,
					Provisioned: &prov,
					Scope:       p,
				})
			}
		}
	}
	return nil
}

func isGoNode(n *schema.Node) bool {
	if n.ServiceFramework == schema.Framework_GO_GRPC {
		return true
	}

	if n.InitializerFor(schema.Framework_GO_GRPC) != nil {
		return true
	}

	for _, pr := range n.Provides {
		for _, available := range pr.AvailableIn {
			if available.Go != nil {
				return true
			}
		}
	}

	return false
}

func makeDep(ctx context.Context, loader workspace.Packages, dep *schema.Instantiate, produceSerialized bool, prov *typeProvider) error {
	pkg, err := loader.LoadByName(ctx, schema.PackageName(dep.PackageName))
	if err != nil {
		return fnerrors.UserError(nil, "failed to load %s/%s: %w", dep.PackageName, dep.Type, err)
	}

	// XXX Well, yes, this shouldn't live here. But being practical. We need to either have
	// a way to define how to generate the types. Or we need to use generics (although generics
	// don't replace all of the uses).
	if shared.IsStdGrpcExtension(dep.PackageName, dep.Type) {
		grpcClientType, err := shared.PrepareGrpcBackendDep(ctx, loader, dep)
		if err != nil {
			return err
		}

		// XXX not hermetic.
		gopkg, err := gosupport.ComputeGoPackage(grpcClientType.Location.Abs())
		if err != nil {
			return err
		}

		clientType := grpcClientType.Name

		prov.GoPackage = gopkg
		prov.Method = "New" + clientType
		prov.Args = append(prov.Args, gosupport.MakeGoPubVar(dep.Name+"Conn"))

		prov.DepVars = append(prov.DepVars, gosupport.TypeDef{
			GoName:      gosupport.MakeGoPubVar(dep.Name),
			GoImportURL: gopkg,
			GoTypeName:  clientType,
		})
		return nil
	}

	_, p := workspace.FindProvider(pkg, schema.PackageName(dep.PackageName), dep.Type)
	if p == nil {
		return fnerrors.UserError(nil, "didn't find a provider for %s/%s", dep.PackageName, dep.Type)
	}

	var goprovider *schema.Provides_AvailableIn_Go
	for _, prov := range p.AvailableIn {
		if prov.Go != nil {
			goprovider = prov.Go
			break
		}
	}

	if goprovider == nil {
		return fnerrors.UserError(nil, "%s: not available for Go", dep.Name)
	}

	prov.Provides = p
	prov.PackageName = schema.PackageName(dep.PackageName)
	prov.GoPackage, _ = packageFrom(pkg.Location)
	prov.Method = makeProvidesMethod(p)

	goImport := goprovider.Package
	if goImport == "" {
		goImport = prov.GoPackage
	}

	prov.DepVars = append(prov.DepVars, gosupport.TypeDef{
		GoName:      gosupport.MakeGoPubVar(dep.Name),
		GoImportURL: goImport,
		GoTypeName:  goprovider.Type,
	})

	if produceSerialized {
		return serializeContents(ctx, loader, p, dep, prov)
	}

	return nil
}

func makeProvidesMethod(p *schema.Provides) string {
	return "Provide" + gosupport.MakeGoPubVar(p.Name)
}

func makeProvidesDepsType(p *schema.Provides) string {
	return gosupport.MakeGoPubVar(p.Name) + "Deps"
}

func serializeContents(ctx context.Context, loader workspace.Packages, provides *schema.Provides, instance *schema.Instantiate, prov *typeProvider) error {
	pkg, err := loader.LoadByName(ctx, schema.PackageName(instance.PackageName))
	if err != nil {
		return err
	}

	parsed, ok := pkg.Provides[provides.Name]
	if !ok {
		return fnerrors.InternalError("%s: protos were not loaded as expected?", instance.PackageName)
	}

	files, msgdesc, err := protos.LoadMessageByName(parsed, provides.Type.Typename)
	if err != nil {
		return fnerrors.InternalError("%s: failed to load message %q: %w", instance.PackageName, provides.Type.Typename, err)
	}

	raw := dynamicpb.NewMessage(msgdesc)
	if err := proto.Unmarshal(instance.Constructor.Value, raw.Interface()); err != nil {
		return fnerrors.InternalError("failed to unmarshal constructor: %w", err)
	}

	// Clean up all values which are not meant to be shipped into the binary.
	protos.CleanupForNonProvisioning(raw)

	deterministicBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(raw.Interface())
	if err != nil {
		return fnerrors.InternalError("failed to marshal depvar: %w", err)
	}

	prov.SerializedMsg = base64.StdEncoding.EncodeToString(deterministicBytes)

	resolver, err := protos.AsResolver(files)
	if err != nil {
		return fnerrors.InternalError("failed to create resolver: %w", err)
	}

	serialized, err := prototext.MarshalOptions{Multiline: true, Resolver: resolver}.Marshal(raw.Interface())
	if err == nil {
		stableFmt, err := parser.Format(serialized)
		if err == nil {
			var b bytes.Buffer
			_, _ = text.NewIndentWriter(&b, []byte("// ")).Write(stableFmt)
			prov.ProtoComments = b.String()
		}
	}

	return nil
}
