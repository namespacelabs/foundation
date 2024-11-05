// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"io/fs"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"github.com/docker/go-units"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/jsonreparser"
	"namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/codegen/protos/fnany"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/binary"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime/constants"
	"namespacelabs.dev/foundation/std/types"
)

type cueProto struct {
	Typename    string   `json:"typename"`
	Sources     []string `json:"source"`
	SkipCodegen bool     `json:"skip_codegen"`
}

type cueExportMethods struct {
	Service cueProto `json:"service"`
	Methods []string `json:"methods"`
}

type cueHttpPath struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Protocol string `json:"protocol"`
}

type cueServerRef struct {
	PackageName string `json:"packageName"`
}

type cueImageRef struct {
	Image string `json:"image"`
}

type cueRequiredStorage struct {
	ByteCount    string `json:"byteCount"`
	MountPath    string `json:"mountPath"`
	PersistentID string `json:"persistentId"`
	Template     bool   `json:"template"`
	StorageClass string `json:"storage_class"`
}

type cueProvides struct {
	Name        string                 `json:"name"`
	Input       *schema.TypeDef        `json:"input"`
	Type        *schema.TypeDef        `json:"type"`
	AvailableIn map[string]interface{} `json:"availableIn"`
	Instantiate cueInstantiate         `json:"instantiate"`
}

type cueInstantiate struct {
	PackageName string   `json:"packageName"`
	Type        string   `json:"type"`
	TypeDef     cueProto `json:"typeDefinition"`
}

type cueCallback struct {
	InvokeInternal string `json:"invokeInternal"`
}

type cueEnvironmentRequirements struct {
	RequiredLabels map[string]string `json:"required"`
}

func parseCueNode(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, loc pkggraph.Location, kind schema.Node_Kind, parent, v *fncue.CueV, out *pkggraph.Package) error {
	node := &schema.Node{
		PackageName: loc.PackageName.String(),
		ModuleName:  loc.Module.ModuleName(),
		Kind:        kind,
	}

	if kind == schema.Node_EXTENSION {
		out.Extension = node
	} else if kind == schema.Node_SERVICE {
		out.Service = node
	} else {
		return fnerrors.NewWithLocation(loc, "unknown kind: %v", kind)
	}

	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return err
	}

	// XXX use this block to use a Decode() function, instead of individual path parsing.

	if err := v.LookupPath("import").Val.Decode(&node.Import); err != nil {
		return err
	}

	if fmwk := v.LookupPath("framework"); fmwk.Exists() {
		fmwkStr, err := fmwk.Val.String()
		if err != nil {
			return err
		}

		fmwk, err := parseFramework(loc, fmwkStr)
		if err != nil {
			return err
		}
		if fmwk == schema.Framework_OPAQUE {
			return fnerrors.NewWithLocation(loc, "Only servers can be OPAQUE")
		}
		node.ServiceFramework = fmwk
	}

	var initializeBefore []string
	if beforeValue := v.LookupPath("initializeBefore"); beforeValue.Exists() {
		if err := beforeValue.Val.Decode(&initializeBefore); err != nil {
			return err
		}
	}

	var initializeAfter []string
	if afterValue := v.LookupPath("initializeAfter"); afterValue.Exists() {
		if err := afterValue.Val.Decode(&initializeAfter); err != nil {
			return err
		}
	}

	var postInitializer string
	if post := v.LookupPath("hasPostInitializerIn"); post.Exists() {
		if err := post.Val.Decode(&postInitializer); err != nil {
			return err
		}
	}

	var initializeInFrameworks []string
	if initializers := v.LookupPath("hasInitializerIn"); initializers.Exists() {
		if err := initializers.Val.Decode(&initializeInFrameworks); err != nil {
			fmwkStr, err := initializers.Val.String()
			if err != nil {
				return err
			}

			initializeInFrameworks = []string{fmwkStr}
		}

		frameworks := uniquestrings.List{}
		for _, fmwkStr := range initializeInFrameworks {
			if !frameworks.Add(fmwkStr) {
				return fnerrors.NewWithLocation(loc, "Duplicate initialization framework value: %s", fmwkStr)
			}

			v, err := parseFramework(loc, fmwkStr)
			if err != nil {
				return err
			}

			node.Initializers = append(node.Initializers, &schema.NodeInitializer{
				Framework:          schema.Framework(v),
				InitializeBefore:   initializeBefore,
				InitializeAfter:    initializeAfter,
				HasPostInitializer: postInitializer == fmwkStr,
			})
		}
	} else {
		if len(initializeBefore) > 0 {
			return fnerrors.NewWithLocation(loc, "initializeBefore can only be set when hasInitializerIn is also set")
		}
		if len(initializeAfter) > 0 {
			return fnerrors.NewWithLocation(loc, "initializeAfter can only be set when hasInitializerIn is also set")
		}

		if postInitializer != "" {
			return fnerrors.NewWithLocation(loc, "hasPostInitializerIn can only be set when hasInitializerIn is also set")
		}
	}

	if provides := v.LookupPath("provides"); provides.Exists() {
		if err := handleProvides(ctx, pl, loc, provides, out, node); err != nil {
			return err
		}
	}

	sort.Slice(node.Provides, func(i, j int) bool {
		return strings.Compare(node.Provides[i].Name, node.Provides[j].Name) < 0
	})

	if packageData := v.LookupPath("packageData"); packageData.Exists() {
		var paths []string
		if err := packageData.Val.Decode(&paths); err != nil {
			return err
		}

		fsys, err := pl.WorkspaceOf(ctx, loc.Module)
		if err != nil {
			return err
		}

		for _, path := range paths {
			contents, err := fs.ReadFile(fsys, loc.Rel(path))
			if err != nil {
				return fnerrors.NewWithLocation(loc, "failed to load eval data %q: %w", path, err)
			}

			out.PackageData = append(out.PackageData, &types.Resource{
				Path:     path,
				Contents: contents,
			})
		}
	}

	if instantiate := v.LookupPath("instantiate"); instantiate.Exists() {
		it, err := instantiate.Val.Fields()
		if err != nil {
			return err
		}

		for it.Next() {
			name := it.Label()

			var inst cueInstantiate

			v := (&fncue.CueV{Val: it.Value()})
			newAPI := false
			if newDefinition := v.LookupPath("#Definition"); newDefinition.Exists() {
				if err := newDefinition.Val.Decode(&inst); err != nil {
					return err
				}
				newAPI = true
			} else {
				// Backwards compatibility with fn_api<20.
				if err := it.Value().Decode(&inst); err != nil {
					return err
				}
			}

			if inst.PackageName != "" {
				node.Reference = append(node.Reference, &schema.Reference{
					PackageName: inst.PackageName,
				})
			}

			constructor, err := constructAny(ctx, inst, v, newAPI, pl, loc)
			if err != nil {
				return err
			}

			node.Instantiate = append(node.Instantiate, &schema.Instantiate{
				PackageName: inst.PackageName,
				Type:        inst.Type,
				Name:        name,
				Constructor: constructor,
			})
		}
	}

	if ingress := v.LookupPath("ingress"); ingress.Exists() {
		v, err := ingress.Val.String()
		if err != nil {
			return err
		}
		node.Ingress = schema.Endpoint_Type(schema.Endpoint_Type_value[v])
	}

	if e := v.LookupPath("exportServicesAsHttp"); e.Exists() {
		_, err := e.Val.Bool()
		if err != nil {
			return err
		}

		return fnerrors.New("exportServicesAsHttp is no longer supported")
	}

	if exported := v.LookupPath("exportService"); exported.Exists() {
		var svc cueProto
		if err := exported.Val.Decode(&svc); err != nil {
			return err
		}

		if err := handleService(ctx, env, pl, loc, cueExportMethods{Service: svc}, node, out); err != nil {
			return err
		}
	}

	if listener := v.LookupPath("listener"); listener.Exists() {
		if err := listener.Val.Decode(&node.ListenerName); err != nil {
			return err
		}
	}

	if exportedPort := v.LookupPath("exportedPort"); exportedPort.Exists() {
		if err := exportedPort.Val.Decode(&node.ExportedPort); err != nil {
			return err
		}
	}

	if exported := v.LookupPath("exportServices"); exported.Exists() {
		var services []cueProto
		if err := exported.Val.Decode(&services); err != nil {
			return err
		}

		for _, svc := range services {
			if err := handleService(ctx, env, pl, loc, cueExportMethods{Service: svc}, node, out); err != nil {
				return err
			}
		}
	}

	if exported := v.LookupPath("exportMethods"); exported.Exists() {
		var export cueExportMethods
		if err := exported.Val.Decode(&export); err != nil {
			return err
		}

		if err := handleService(ctx, env, pl, loc, export, node, out); err != nil {
			return err
		}
	}

	if exported := v.LookupPath("exportHttp"); exported.Exists() {
		var paths []cueHttpPath
		if err := exported.Val.Decode(&paths); err != nil {
			return err
		}

		for _, p := range paths {
			path := &schema.HttpPath{
				Path: p.Path,
				Kind: p.Kind,
			}

			if p.Protocol != "" {
				v, ok := schema.IngressFragment_IngressHttpPath_BackendProtocol_value[strings.ToUpper(p.Protocol)]
				if !ok {
					return fnerrors.NewWithLocation(loc, "unrecognized protocol %q", p.Protocol)
				}

				path.BackendProtocol = schema.IngressFragment_IngressHttpPath_BackendProtocol(v)
			}

			node.ExportHttp = append(node.ExportHttp, path)
		}
	}

	if exported := v.LookupPath("requirePersistentStorage"); exported.Exists() {
		var d cueRequiredStorage
		if err := exported.Val.Decode(&d); err != nil {
			return fnerrors.NewWithLocation(loc, "failed to parse: %w", err)
		}

		if d.PersistentID == "" {
			return fnerrors.NewWithLocation(loc, "persistentId is required")
		}

		v, err := units.FromHumanSize(d.ByteCount)
		if err != nil {
			return fnerrors.NewWithLocation(loc, "failed to parse value: %w", err)
		}

		pv, err := anypb.New(&schema.PersistentVolume{
			Id:           d.PersistentID,
			SizeBytes:    uint64(v),
			Template:     d.Template,
			StorageClass: d.StorageClass,
		})
		if err != nil {
			return fnerrors.NewWithLocation(loc, "failed to marshal persistent volume: %w", err)
		}

		node.Volume = append(node.Volume, &schema.Volume{
			Owner:      node.PackageName,
			Name:       d.PersistentID,
			Kind:       constants.VolumeKindPersistent,
			Definition: pv,
		})

		node.Mount = append(node.Mount, &schema.Mount{
			Owner:     node.PackageName,
			Path:      d.MountPath,
			VolumeRef: schema.MakePackageRef(loc.PackageName, d.PersistentID),
		})
	}

	if mounts := v.LookupPath("mounts"); mounts.Exists() {
		parsedMounts, inlinedVolumes, err := ParseMounts(ctx, pl, loc, mounts)
		if err != nil {
			return fnerrors.NewWithLocation(loc, "parsing mounts: %w", err)
		}

		node.Volume = append(node.Volume, inlinedVolumes...)
		node.Mount = parsedMounts
	}

	if environment := v.LookupPath("environment"); environment.Exists() {
		var er cueEnvironmentRequirements
		if err := environment.Val.Decode(&er); err != nil {
			return fnerrors.NewWithLocation(loc, "failed to parse: %w", err)
		}

		node.EnvironmentRequirement = &schema.Node_EnvironmentRequirement{}
		for k, v := range er.RequiredLabels {
			node.EnvironmentRequirement.EnvironmentHasLabel = append(node.EnvironmentRequirement.EnvironmentHasLabel, &schema.Label{
				Name:  k,
				Value: v,
			})
		}

		node.EnvironmentRequirement.EnvironmentHasLabel = sortLabels(node.EnvironmentRequirement.EnvironmentHasLabel)
	}

	if on := v.LookupPath("on.prepare"); on.Exists() {
		var callback cueCallback
		if err := on.Val.Decode(&callback); err != nil {
			return fnerrors.NewWithLocation(loc, "failed to parse `on.provision`: %w", err)
		}

		binInvocation, err := binary.ParseBinaryInvocationField(ctx, env, pl, out, "genb-node-inv" /* binaryName */, "invokeBinary" /* cuePath */, on)
		if err != nil {
			return fnerrors.NewWithLocation(loc, "failed to parse `on.provision.invokeBinary`: %w", err)
		}

		if callback.InvokeInternal == "" {
			if binInvocation == nil {
				return fnerrors.NewWithLocation(loc, "on.provision.invokeInternal or on.provision.invokeBinary is required")
			}
		} else {
			if binInvocation != nil {
				return fnerrors.NewWithLocation(loc, "on.provision.invokeInternal and on.provision.invokeBinary are exclusive")
			}
		}

		out.PrepareHooks = append(out.PrepareHooks, pkggraph.PrepareHook{
			InvokeInternal: callback.InvokeInternal,
			InvokeBinary:   binInvocation,
		})
	}

	sort.Slice(node.Instantiate, func(i, j int) bool {
		return strings.Compare(node.Instantiate[i].Name, node.Instantiate[j].Name) < 0
	})

	if r := v.LookupPath("resources"); r.Exists() {
		resourceList, err := ParseResourceList(r)
		if err != nil {
			return fnerrors.NewWithLocation(loc, "parsing resources failed: %w", err)
		}

		pack, err := resourceList.ToPack(ctx, env, pl, out)
		if err != nil {
			return err
		}

		node.ResourcePack = pack
	}

	if r := v.LookupPath("extensions"); r.Exists() {
		if err := r.Val.Decode(&node.Extension); err != nil {
			return err
		}
	}

	if err := fncue.WalkAttrs(parent.Val, func(v cue.Value, key, value string) error {
		switch key {
		case fncue.InputKeyword:
			if err := handleRef(loc, v, value, &node.Reference); err != nil {
				return err
			}

		case fncue.AllocKeyword:
			need := &schema.Need{
				CuePath: v.Path().String(),
			}
			node.Need = append(node.Need, need)

			switch value {
			case fncue.ServerPortAllocKw:
				portName, err := v.LookupPath(cue.ParsePath("name")).String()
				if err != nil {
					return err
				}

				need.Type = &schema.Need_Port_{Port: &schema.Need_Port{Name: portName}}

			default:
				return fnerrors.InternalError("don't know need %q", value)
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return parsing.TransformNode(ctx, pl, loc, node, kind)
}

func sortLabels(labels []*schema.Label) []*schema.Label {
	slices.SortFunc(labels, func(a, b *schema.Label) int {
		if a.GetName() == b.GetName() {
			return strings.Compare(a.GetValue(), b.GetValue())
		}
		return strings.Compare(a.GetName(), b.GetName())
	})
	return labels
}

func handleService(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, loc pkggraph.Location, export cueExportMethods, node *schema.Node, out *pkggraph.Package) error {
	fsys, err := pl.WorkspaceOf(ctx, loc.Module)
	if err != nil {
		return err
	}

	parseOpts, err := parsing.MakeProtoParseOpts(ctx, pl, loc.Module.Workspace)
	if err != nil {
		return err
	}

	parsed, err := parseOpts.ParseAtLocation(fsys, loc, export.Service.Sources)
	if err != nil {
		return fnerrors.NewWithLocation(loc, "failed to parse proto sources %v: %v", export.Service.Sources, err)
	}

	_, desc, err := protos.LoadDescriptorByName(parsed, export.Service.Typename)
	if err != nil {
		return fnerrors.NewWithLocation(loc, "failed to load service %q: %v", export.Service.Typename, err)
	}

	svc, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return fnerrors.NewWithLocation(loc, "expected %q to be a service: %v", export.Service.Typename, err)
	}

	// Validated that the methods exported are actually part of the service.
	if len(export.Methods) > 0 {
		var notFound []string
		for _, method := range export.Methods {
			// XXX O(n^2)
			var found bool
			for i := 0; i < svc.Methods().Len(); i++ {
				declared := svc.Methods().Get(i)
				if string(declared.Name()) == method {
					found = true
					break
				}
			}
			if !found {
				notFound = append(notFound, method)
			}
		}
		if len(notFound) > 0 {
			return fnerrors.NewWithLocation(loc, "%s: the following methods don't exist in the service: %v", export.Service.Typename, notFound)
		}
	}

	node.ExportService = append(node.ExportService, &schema.GrpcExportService{
		ProtoTypename:            export.Service.Typename,
		Proto:                    export.Service.Sources,
		Method:                   export.Methods,
		ServerReflectionIncluded: env.GetPurpose() != schema.Environment_PRODUCTION,
	})

	if out.Services == nil {
		out.Services = map[string]*protos.FileDescriptorSetAndDeps{}
	}

	out.Services[export.Service.Typename] = parsed
	if export.Service.SkipCodegen {
		out.SkipServiceProtogen = append(out.SkipServiceProtogen, export.Service.Typename)
	}
	return nil
}

func handleProvides(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, provides *fncue.CueV, pkg *pkggraph.Package, out *schema.Node) error {
	it, err := provides.Val.Fields()
	if err != nil {
		return err
	}

	parseOpts, err := parsing.MakeProtoParseOpts(ctx, pl, loc.Module.Workspace)
	if err != nil {
		return err
	}

	for it.Next() {
		name := it.Label()

		var provides cueProvides
		if err := it.Value().Decode(&provides); err != nil {
			return err
		}
		p := &schema.Provides{
			Name: provides.Name,
			Type: provides.Type,
		}

		if provides.Input != nil {
			if provides.Type != nil {
				return fnerrors.NewWithLocation(loc, "can't specify both input and type in a provides block")
			}
			p.Type = provides.Input
		} else if provides.Type != nil {
			p.Type = provides.Type
		} else {
			return fnerrors.NewWithLocation(loc, "a provides block requires a input definition")
		}

		fsys, err := pl.WorkspaceOf(ctx, loc.Module)
		if err != nil {
			return err
		}

		parsed, err := parseOpts.ParseAtLocation(fsys, loc, p.Type.Source)
		if err != nil {
			return fnerrors.NewWithLocation(loc, "failed to parse proto sources %v: %v", p.Type.Source, err)
		}

		if _, _, err := protos.LoadMessageByName(parsed, p.Type.Typename); err != nil {
			return fnerrors.NewWithLocation(loc, "failed to load message %q: %v", p.Type.Typename, err)
		}

		if pkg.Provides == nil {
			pkg.Provides = map[string]*protos.FileDescriptorSetAndDeps{}
		}
		pkg.Provides[name] = parsed

		keys := maps.Keys(provides.AvailableIn)
		slices.Sort(keys)

		for _, k := range keys {
			m := provides.AvailableIn[k]

			// XXX This should use reflection.
			switch k {
			case "go":
				g := &schema.Provides_AvailableIn_Go{}
				if err := jsonreparser.Reparse(m, g); err != nil {
					return fnerrors.NewWithLocation(loc, "failed to reparse: %w", err)
				}
				p.AvailableIn = append(p.AvailableIn, &schema.Provides_AvailableIn{
					Go: g,
				})
			}
		}

		v := fncue.CueV{Val: it.Value()}
		if instantiate := v.LookupPath("instantiate"); instantiate.Exists() {
			it, err := instantiate.Val.Fields()
			if err != nil {
				return err
			}

			for it.Next() {
				name := it.Label()

				var inst cueInstantiate

				v := (&fncue.CueV{Val: it.Value()})
				newAPI := false
				if newDefinition := v.LookupPath("#Definition"); newDefinition.Exists() {
					if err := newDefinition.Val.Decode(&inst); err != nil {
						return err
					}
					newAPI = true
				} else {
					// Backwards compatibility with fn_api<20.
					if err := it.Value().Decode(&inst); err != nil {
						return err
					}
				}

				if inst.PackageName != "" {
					out.Reference = append(out.Reference, &schema.Reference{
						PackageName: inst.PackageName,
					})
				}

				constructor, err := constructAny(ctx, inst, v, newAPI, pl, loc)
				if err != nil {
					return err
				}

				p.Instantiate = append(p.Instantiate, &schema.Instantiate{
					PackageName: inst.PackageName,
					Type:        inst.Type,
					Name:        name,
					Constructor: constructor,
				})
			}
		}

		out.Provides = append(out.Provides, p)
	}

	return nil
}

func constructAny(ctx context.Context, inst cueInstantiate, v *fncue.CueV, newAPI bool, pl parsing.EarlyPackageLoader, loc pkggraph.Location) (*anypb.Any, error) {
	if inst.PackageName == "" {
		if len(inst.TypeDef.Sources) > 0 {
			return nil, fnerrors.NewWithLocation(loc, "source can't be provided when package is unspecified")
		}

		msgtype, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(inst.TypeDef.Typename))
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "%s: no such message: %w", inst.TypeDef.Typename, err)
		}

		var msg proto.Message
		if newAPI {
			msg, err = v.DecodeAs(msgtype)
		} else {
			msg, err = v.LookupPath("with").DecodeAs(msgtype)
		}
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "%s: failed to decode builtin message: %w", inst.TypeDef.Typename, err)
		}

		constructor, err := anypb.New(msg)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "%s: failed to serialize constructor: %w", inst.TypeDef.Typename, err)
		}
		return constructor, nil
	}

	pkg, err := pl.LoadByName(ctx, schema.PackageName(inst.PackageName))
	if err != nil {
		return nil, err
	}

	resolved := pkg.Location
	fsys, err := pl.WorkspaceOf(ctx, pkg.Location.Module)
	if err != nil {
		return nil, err
	}

	opts, err := parsing.MakeProtoParseOpts(ctx, pl, loc.Module.Workspace)
	if err != nil {
		return nil, err
	}

	msgdesc, err := opts.LoadMessageAtLocation(fsys, resolved, inst.TypeDef.Sources, inst.TypeDef.Typename)
	if err != nil {
		return nil, fnerrors.NewWithLocation(loc, "%s: %w", resolved.PackageName, err)
	}

	var msg proto.Message
	if newAPI {
		msg, err = v.DecodeAs(dynamicpb.NewMessageType(msgdesc))
	} else {
		msg, err = v.LookupPath("with").DecodeAs(dynamicpb.NewMessageType(msgdesc))
	}
	if err != nil {
		return nil, fnerrors.NewWithLocation(loc, "%s: %s: failed to decode message: %w", resolved.PackageName, inst.TypeDef.Typename, err)
	}

	return fnany.Marshal(resolved.PackageName, msg)
}

func handleRef(loc pkggraph.Location, v cue.Value, value string, out *[]*schema.Reference) error {
	switch value {
	case fncue.ProtoloadIKw:
		var ref cueProtoload
		if err := v.Decode(&ref); err != nil {
			return err
		}

		// Transform all source references to be relative to the module.
		var sources []string
		for _, src := range ref.Sources {
			p, err := loc.CheckRel(src)
			if err != nil {
				return err
			}
			sources = append(sources, p)
		}

		if len(ref.Sources) > 0 {
			*out = append(*out, &schema.Reference{
				CuePath:  v.Path().String(),
				FilePath: sources,
				Kind:     schema.Reference_PROTO_DEPS,
			})
		}

	case fncue.ServerDepIKw:
		var ref cueServerRef
		if err := v.Decode(&ref); err != nil {
			return err
		}

		if ref.PackageName != "" {
			*out = append(*out, &schema.Reference{
				CuePath:     v.Path().String(),
				PackageName: ref.PackageName,
				Kind:        schema.Reference_SERVER,
			})
		}

	case fncue.ImageIKw:
		var ref cueImageRef
		if err := v.Decode(&ref); err != nil {
			return err
		}

		if ref.Image != "" {
			*out = append(*out, &schema.Reference{
				CuePath: v.Path().String(),
				Image:   ref.Image,
				Kind:    schema.Reference_IMAGE,
			})
		}
	}

	return nil
}
