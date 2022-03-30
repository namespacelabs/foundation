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
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

type nodeLoc struct {
	Location workspace.Location
	Node     *schema.Node
}

type instancedDepList struct {
	services     []nodeLoc
	instances    []*instancedDep
	initializers []nodeLoc
}

type instancedDep struct {
	Location    workspace.Location
	Parent      *schema.Node
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

func expandInstancedDeps(ctx context.Context, loader workspace.Packages, computedIncludes []schema.PackageName, nodes []*schema.Node) (instancedDepList, error) {
	var e instancedDepList

	m := map[string]*schema.Node{}
	for _, n := range nodes {
		m[n.PackageName] = n
	}

	for _, ref := range computedIncludes {
		referenced := m[ref.String()]
		if referenced == nil {
			return e, fnerrors.InternalError("%s: package not loaded?", ref)
		}

		refLoc, err := loader.Resolve(ctx, ref)
		if err != nil {
			return e, err
		}

		if err := expandNode(ctx, loader, refLoc, referenced, true, &e); err != nil {
			return e, err
		}

		if referenced.GetKind() == schema.Node_SERVICE {
			e.services = append(e.services, nodeLoc{Location: refLoc, Node: referenced})
		}

		if referenced.GetInitializer(schema.Framework_GO) != nil {
			e.initializers = append(e.initializers, nodeLoc{Location: refLoc, Node: referenced})
		}
	}

	return e, nil
}

func visitAllDeps(ctx context.Context, nodes []*schema.Node, includes []schema.PackageName, visitor func(*schema.Node) error) error {
	m := map[string]*schema.Node{}
	for _, n := range nodes {
		m[n.PackageName] = n
	}

	for _, ref := range includes {
		referenced := m[ref.String()]
		if referenced == nil {
			return fnerrors.InternalError("%s: package not loaded", ref)
		}

		if err := visitor(referenced); err != nil {
			return err
		}
	}

	return nil
}

func expandNode(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, produceSerialized bool, e *instancedDepList) error {
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

	return nil
}

func makeDep(ctx context.Context, loader workspace.Packages, dep *schema.Instantiate, produceSerialized bool, prov *typeProvider) error {
	ptype, err := workspace.ResolveDependency(dep)
	if err != nil {
		return err
	}

	if ptype.Builtin {
		switch ptype.ProtoType {
		case "foundation.languages.golang.Instantiate":
			inst := &Instantiate{}
			if err := proto.Unmarshal(dep.Constructor.Value, inst); err != nil {
				return fnerrors.UserError(nil, "failed to unmarshal %s: %w", dep.Constructor.TypeUrl, err)
			}

			prov.GoPackage = inst.Package
			prov.Method = inst.Method
			for _, arg := range inst.Arguments {
				prov.Args = append(prov.Args, gosupport.MakeGoPubVar(arg.Ref))
			}

			prov.DepVars = append(prov.DepVars, gosupport.TypeDef{
				GoName:      gosupport.MakeGoPubVar(dep.Name),
				GoImportURL: inst.Package,
				GoTypeName:  inst.Typename,
			})

		default:
			return fnerrors.UserError(nil, "don't know how to instantiate %v", dep)
		}

		return nil
	}

	pkg, err := loader.LoadByName(ctx, ptype.Package)
	if err != nil {
		return fnerrors.UserError(nil, "failed to load %s (for %s): %w", ptype.Package, ptype.ProtoType, err)
	}

	_, p := workspace.FindProvider(pkg, ptype.Package, ptype.ProtoType)
	if p == nil {
		return fnerrors.UserError(nil, "didn't find a provider for %s/%s", ptype.Package, ptype.ProtoType)
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
	prov.PackageName = ptype.Package
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

	msg := dynamicpb.NewMessage(msgdesc).Interface()
	if err := proto.Unmarshal(instance.Constructor.Value, msg); err != nil {
		return fnerrors.InternalError("failed to unmarshal constructor: %w", err)
	}

	deterministicBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
	if err != nil {
		return fnerrors.InternalError("failed to marshal depvar: %w", err)
	}

	prov.SerializedMsg = base64.StdEncoding.EncodeToString(deterministicBytes)

	resolver, err := protos.AsResolver(files)
	if err != nil {
		return fnerrors.InternalError("failed to create resolver: %w", err)
	}

	serialized, err := prototext.MarshalOptions{Multiline: true, Resolver: resolver}.Marshal(msg)
	if err == nil {
		stableFmt, err := parser.Format(serialized)
		if err == nil {
			var b bytes.Buffer
			text.NewIndentWriter(&b, []byte("// ")).Write(stableFmt)
			prov.ProtoComments = b.String()
		}
	}

	return nil
}
