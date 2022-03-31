// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func TransformNode(ctx context.Context, pl Packages, loc Location, node *schema.Node, kind schema.Node_Kind) error {
	if kind == schema.Node_EXTENSION {
		if node.Ingress != schema.Endpoint_INGRESS_UNSPECIFIED {
			return errors.New("ingress can only be specified for services")
		}

		if len(node.ExportService) > 0 {
			return errors.New("extensions can't export services")
		}
	}

	var deps schema.PackageList
	for k, dep := range node.Instantiate {
		// Checking language compatibility

		// No package happen for internal nodes, e.g. "std/grpc"
		if len(dep.PackageName) > 0 {
			pkg, err := pl.LoadByName(ctx, schema.PackageName(dep.PackageName))
			if err != nil {
				return err
			}
			ext := pkg.Extension
			if ext == nil {
				return fnerrors.UserError(loc, "Trying to instantiate a node that is not an extension: %s", dep.PackageName)
			}
			providesFmwks := ext.GetProvidesFrameworks()
			for _, fmwk := range node.GetCodegenFrameworks() {
				if _, ok := providesFmwks[fmwk]; !ok {
					return fnerrors.UserError(
						loc,
						"Node has generated code for framework %s but tries to instantiate an "+
							"extension provider '%s:%s' that doesn't support this framework",
						fmwk.String(),
						dep.PackageName,
						dep.Name,
					)
				}
			}
		}

		// Adding a proto dependency.

		ptype, err := ResolveDependency(dep)
		if err != nil {
			return fnerrors.Wrapf(loc, err, "dep#%d (%s): %w", k, dep.Name)
		}

		if !ptype.Builtin {
			deps.Add(ptype.Package)
		}
	}

	if kind == schema.Node_SERVICE {
		node.IngressServiceName = filepath.Base(loc.PackageName.String())
	}

	for _, imp := range node.Import {
		deps.Add(schema.PackageName(imp))
	}

	node.UserImports = deps.PackageNamesAsString()

	err := visitDeps(ctx, pl, loc, deps.PackageNames(), &deps, nil)
	if err != nil {
		return err
	}

	// XXX stable order is missing
	for _, handler := range FrameworkHandlers {
		var ext FrameworkExt
		if err := handler.ParseNode(ctx, loc, &ext); err != nil {
			return err
		}

		for _, incl := range ext.Include {
			deps.Add(schema.PackageName(incl))
		}

		if ext.FrameworkSpecific != nil {
			node.Ext = append(node.Ext, ext.FrameworkSpecific)
		}
	}

	node.Import = deps.PackageNamesAsString()
	return nil
}

type ParsedType struct {
	Package   schema.PackageName
	ProtoType string
	Builtin   bool
}

func ResolveDependency(dep *schema.Instantiate) (ParsedType, error) {
	if dep.Constructor != nil {
		if t := strings.TrimPrefix(dep.Constructor.TypeUrl, "type.googleapis.com/"); t != dep.Constructor.TypeUrl {
			if dep.PackageName != "" {
				return ParsedType{
					Package:   schema.PackageName(dep.PackageName),
					ProtoType: t,
				}, nil
			}

			return ParsedType{
				ProtoType: t,
				Builtin:   true,
			}, nil
		}

		if t := strings.TrimPrefix(dep.Constructor.TypeUrl, typeUrlBaseSlash); t != dep.Constructor.TypeUrl {
			return ParsedType{
				Package:   schema.PackageName(filepath.Dir(t)),
				ProtoType: filepath.Base(t),
			}, nil
		}
	}

	return ParsedType{}, fnerrors.InternalError("don't know how to build type")
}

func visitDeps(ctx context.Context, pl Packages, loc Location, includes []schema.PackageName, dl *schema.PackageList, visit func(n *schema.Node) error) error {
	for _, include := range includes {
		dep, err := loadDep(ctx, pl, include)
		if err != nil {
			return fnerrors.Wrapf(loc, err, "loading dependency: %s", include)
		}

		for _, v := range dep.Node().GetImport() {
			dl.Add(schema.PackageName(v))
		}

		if visit != nil {
			if err := visit(dep.Node()); err != nil {
				return err
			}
		}

		dl.Add(include)
	}

	return nil
}

func loadDep(ctx context.Context, pl Packages, pkg schema.PackageName) (*Package, error) {
	p, err := pl.LoadByName(ctx, pkg)
	if err != nil {
		return nil, err
	}

	if p.Server != nil {
		return nil, errors.New("dependencies can't include servers")
	}

	if p.Binary != nil {
		return nil, errors.New("dependencies can't be binaries")
	}

	return p, nil
}
