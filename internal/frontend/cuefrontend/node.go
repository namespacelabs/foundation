// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"encoding/json"
	"io/fs"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"github.com/docker/go-units"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos"
	"namespacelabs.dev/foundation/workspace/source/protos/fnany"
)

type cueGrpcService struct {
	Typename string   `json:"typename"`
	Sources  []string `json:"source"`
}

type cueHttpPath struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
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
}

type cueProvides struct {
	Name        string                 `json:"name"`
	Input       *schema.TypeDef        `json:"input"`
	Type        *schema.TypeDef        `json:"type"`
	AvailableIn map[string]interface{} `json:"availableIn"`
}

type cueInstantiate struct {
	PackageName string   `json:"packageName"`
	Type        string   `json:"type"`
	TypeDef     cueProto `json:"typeDefinition"`
}

func parseCueNode(ctx context.Context, pl workspace.EarlyPackageLoader, loc workspace.Location, kind schema.Node_Kind, parent, v *fncue.CueV, pkg *workspace.Package, opts workspace.LoadPackageOpts) error {
	out := &schema.Node{
		PackageName: loc.PackageName.String(),
		Kind:        kind,
	}

	if kind == schema.Node_EXTENSION {
		pkg.Extension = out
	} else if kind == schema.Node_SERVICE {
		pkg.Service = out
	} else {
		return fnerrors.UserError(loc, "unknown kind: %v", kind)
	}

	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return err
	}

	// XXX use this block to use a Decode() function, instead of individual path parsing.

	if err := v.LookupPath("import").Val.Decode(&out.Import); err != nil {
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
			return fnerrors.UserError(loc, "Only servers can be OPAQUE")
		}
		out.ServiceFramework = fmwk
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
		uniqFrameworks := uniquestrings.List{}
		for _, fmwkStr := range initializeInFrameworks {
			if !uniqFrameworks.Add(fmwkStr) {
				return fnerrors.UserError(loc, "Duplicate initialization framework value: %s", fmwkStr)
			}
			v, err := parseFramework(loc, fmwkStr)
			if err != nil {
				return err
			}
			out.Initializers = append(out.Initializers, &schema.NodeInitializer{Framework: schema.Framework(v)})
		}
	}

	if provides := v.LookupPath("provides"); provides.Exists() {
		if err := handleProvides(ctx, pl, loc, provides, pkg, out); err != nil {
			return err
		}
	}

	sort.Slice(out.Provides, func(i, j int) bool {
		return strings.Compare(out.Provides[i].Name, out.Provides[j].Name) < 0
	})

	if packageData := v.LookupPath("packageData"); packageData.Exists() {
		var paths []string
		if err := packageData.Val.Decode(&paths); err != nil {
			return err
		}

		if opts.LoadDependencies {
			fsys, err := pl.WorkspaceOf(ctx, loc.Module)
			if err != nil {
				return err
			}

			for _, path := range paths {
				contents, err := fs.ReadFile(fsys, loc.Rel(path))
				if err != nil {
					return fnerrors.UserError(loc, "failed to load eval data %q: %w", path, err)
				}

				pkg.PackageData = append(pkg.PackageData, &types.Resource{
					Path:     path,
					Contents: contents,
				})
			}
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
				out.Reference = append(out.Reference, &schema.Reference{
					PackageName: inst.PackageName,
				})
			}

			if opts.LoadDependencies {
				var constructor *anypb.Any

				if inst.PackageName == "" {
					if len(inst.TypeDef.Source) > 0 {
						return fnerrors.UserError(loc, "source can't be provided when package is unspecified")
					}

					msgtype, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(inst.TypeDef.Typename))
					if err != nil {
						return fnerrors.UserError(loc, "%s: no such message: %w", inst.TypeDef.Typename, err)
					}

					msg, err := (&fncue.CueV{Val: it.Value()}).LookupPath("with").DecodeAs(msgtype)
					if err != nil {
						return fnerrors.UserError(loc, "%s: failed to decode builtin message: %w", inst.TypeDef.Typename, err)
					}

					constructor, err = anypb.New(msg)
					if err != nil {
						return fnerrors.UserError(loc, "%s: failed to serialize constructor: %w", inst.TypeDef.Typename, err)
					}
				} else {
					pkg, err := pl.LoadByName(ctx, schema.PackageName(inst.PackageName))
					if err != nil {
						return err
					}

					resolved := pkg.Location
					fsys, err := pl.WorkspaceOf(ctx, pkg.Location.Module)
					if err != nil {
						return err
					}

					msgdesc, err := protos.LoadMessageAtLocation(fsys, resolved, inst.TypeDef.Source, inst.TypeDef.Typename)
					if err != nil {
						return fnerrors.UserError(loc, "%s: %w", resolved.PackageName, err)
					}

					var msg proto.Message
					if newAPI {
						msg, err = v.DecodeAs(dynamicpb.NewMessageType(msgdesc))
					} else {
						msg, err = v.LookupPath("with").DecodeAs(dynamicpb.NewMessageType(msgdesc))
					}

					if err != nil {
						return fnerrors.UserError(loc, "%s: %s: failed to decode message: %w", resolved.PackageName, inst.TypeDef.Typename, err)
					}

					constructor, err = fnany.Marshal(resolved.PackageName, msg)
					if err != nil {
						return err // Error already has context.
					}
				}

				out.Instantiate = append(out.Instantiate, &schema.Instantiate{
					PackageName: inst.PackageName,
					Type:        inst.Type,
					Name:        name,
					Constructor: constructor,
				})
			}
		}
	}

	sort.Slice(out.Instantiate, func(i, j int) bool {
		return strings.Compare(out.Instantiate[i].Name, out.Instantiate[j].Name) < 0
	})

	if ingress := v.LookupPath("ingress"); ingress.Exists() {
		v, err := ingress.Val.String()
		if err != nil {
			return err
		}
		out.Ingress = schema.Endpoint_Type(schema.Endpoint_Type_value[v])
	}

	if exported := v.LookupPath("exportService"); exported.Exists() {
		var svc cueGrpcService
		if err := exported.Val.Decode(&svc); err != nil {
			return err
		}

		if e := v.LookupPath("exportServicesAsHttp"); e.Exists() {
			vb, err := e.Val.Bool()
			if err != nil {
				return err
			}
			out.ExportServicesAsHttp = vb
		}

		out.ExportService = append(out.ExportService, &schema.GrpcExportService{
			ProtoTypename: svc.Typename,
			Proto:         svc.Sources,
		})

		fsys, err := pl.WorkspaceOf(ctx, loc.Module)
		if err != nil {
			return err
		}

		parsed, err := protos.ParseAtLocation(fsys, loc, svc.Sources)
		if err != nil {
			return fnerrors.UserError(loc, "failed to parse %v: %v", svc.Sources, err)
		}

		_, desc, err := protos.LoadDescriptorByName(parsed, svc.Typename)
		if err != nil {
			return fnerrors.UserError(loc, "failed to load service %q: %v", svc.Typename, err)
		}

		if _, ok := desc.(protoreflect.ServiceDescriptor); !ok {
			return fnerrors.UserError(loc, "expected %q to be a service: %v", svc.Typename, err)
		}

		if pkg.Services == nil {
			pkg.Services = map[string]*protos.FileDescriptorSetAndDeps{}
		}
		pkg.Services[svc.Typename] = parsed
	}

	if exported := v.LookupPath("exportHttp"); exported.Exists() {
		var paths []cueHttpPath
		if err := exported.Val.Decode(&paths); err != nil {
			return err
		}

		for _, p := range paths {
			out.ExportHttp = append(out.ExportHttp, &schema.HttpPath{
				Path: p.Path,
				Kind: p.Kind,
			})
		}
	}

	if exported := v.LookupPath("requirePersistentStorage"); exported.Exists() {
		var d cueRequiredStorage
		if err := exported.Val.Decode(&d); err != nil {
			return fnerrors.Wrapf(loc, err, "failed to parse")
		}

		if d.PersistentID == "" {
			return fnerrors.UserError(loc, "persistentId is required")
		}

		v, err := units.FromHumanSize(d.ByteCount)
		if err != nil {
			return fnerrors.Wrapf(loc, err, "failed to parse value")
		}

		out.RequiredStorage = append(out.RequiredStorage, &schema.RequiredStorage{
			PersistentId: d.PersistentID,
			ByteCount:    uint64(v),
			MountPath:    d.MountPath,
		})
	}

	if err := fncue.WalkAttrs(parent.Val, func(v cue.Value, key, value string) error {
		switch key {
		case fncue.InputKeyword:
			if err := handleRef(loc, v, value, &out.Reference); err != nil {
				return err
			}

		case fncue.AllocKeyword:
			need := &schema.Need{
				CuePath: v.Path().String(),
			}
			out.Need = append(out.Need, need)

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

	return workspace.TransformNode(ctx, pl, loc, out, kind)
}

func handleProvides(ctx context.Context, pl workspace.EarlyPackageLoader, loc workspace.Location, provides *fncue.CueV, pkg *workspace.Package, out *schema.Node) error {
	m := map[string]cueProvides{}
	if err := provides.Val.Decode(&m); err != nil {
		return err
	}

	var keys uniquestrings.List
	for key := range m {
		keys.Add(key)
	}

	for _, key := range keys.Strings() {
		p := &schema.Provides{
			Name: m[key].Name,
			Type: m[key].Type,
		}

		if m[key].Input != nil {
			if m[key].Type != nil {
				return fnerrors.UserError(loc, "can't specify both input and type in a provides block")
			}
			p.Type = m[key].Input
		} else if m[key].Type != nil {
			p.Type = m[key].Type
		} else {
			return fnerrors.UserError(loc, "a provides block requires a input definition")
		}

		fsys, err := pl.WorkspaceOf(ctx, loc.Module)
		if err != nil {
			return err
		}

		parsed, err := protos.ParseAtLocation(fsys, loc, p.Type.Source)
		if err != nil {
			return fnerrors.UserError(loc, "failed to parse %v: %v", p.Type.Source, err)
		}

		if _, _, err := protos.LoadMessageByName(parsed, p.Type.Typename); err != nil {
			return fnerrors.UserError(loc, "failed to load message %q: %v", p.Type.Typename, err)
		}

		if pkg.Provides == nil {
			pkg.Provides = map[string]*protos.FileDescriptorSetAndDeps{}
		}
		pkg.Provides[key] = parsed

		for k, m := range m[key].AvailableIn {
			// XXX This should use reflection.
			switch k {
			case "go":
				g := &schema.Provides_AvailableIn_Go{}
				remarshal, err := json.Marshal(m)
				if err != nil {
					return fnerrors.UserError(loc, "failed to marshal: %w", err)
				}
				if err := json.Unmarshal(remarshal, g); err != nil {
					return fnerrors.UserError(loc, "failed to unmarshal: %w", err)
				}
				p.AvailableIn = append(p.AvailableIn, &schema.Provides_AvailableIn{
					Go: g,
				})
			case "web":
				{
					p.AvailableIn = append(p.AvailableIn, &schema.Provides_AvailableIn{
						Web: &schema.Provides_AvailableIn_Web{},
					})
				}
			}
		}

		out.Provides = append(out.Provides, p)
	}

	return nil
}

func handleRef(loc workspace.Location, v cue.Value, value string, out *[]*schema.Reference) error {
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
