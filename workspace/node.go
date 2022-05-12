// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func TransformNode(ctx context.Context, pl Packages, loc Location, node *schema.Node, kind schema.Node_Kind, opts LoadPackageOpts) error {
	if kind == schema.Node_EXTENSION {
		if node.Ingress != schema.Endpoint_INGRESS_UNSPECIFIED {
			return fnerrors.New("ingress can only be specified for services")
		}

		if len(node.ExportService) > 0 {
			return fnerrors.New("extensions can't export services")
		}
	}

	var additionalInstances []*schema.Instantiate

	var deps schema.PackageList
	instances := node.Instantiate
	for _, p := range node.Provides {
		instances = append(instances, p.Instantiate...)
	}
	for _, dep := range instances {
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
			providesFmwks := ext.ProvidedInFrameworks()
			for _, fmwk := range node.CodegeneratedFrameworks() {
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
		if ref := protos.Ref(dep.Constructor); ref != nil && !ref.Builtin {
			deps.Add(ref.Package)
		}

		// XXX this special casing should be found a new home.
		if dep.PackageName == "namespacelabs.dev/foundation/std/grpc" && dep.Type == "Backend" {
			additionalInstances = append(additionalInstances, &schema.Instantiate{
				PackageName: dep.PackageName,
				Type:        "Conn",
				Name:        dep.Name + "Conn",
				Constructor: dep.Constructor,
			})
		}
	}

	node.Instantiate = append(node.Instantiate, additionalInstances...)

	if kind == schema.Node_SERVICE {
		node.IngressServiceName = filepath.Base(loc.PackageName.String())
	}

	for _, imp := range node.Import {
		deps.Add(schema.PackageName(imp))
	}

	node.UserImports = deps.PackageNamesAsString()

	if opts.LoadPackageReferences {
		err := visitDeps(ctx, pl, loc, deps.PackageNames(), &deps, nil)
		if err != nil {
			return err
		}
	}

	// XXX stable order is missing
	for _, handler := range FrameworkHandlers {
		var ext FrameworkExt
		if err := handler.ParseNode(ctx, loc, node, &ext); err != nil {
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
		return nil, fnerrors.New("dependencies can't include servers")
	}

	if p.Binary != nil {
		return nil, fnerrors.New("dependencies can't be binaries")
	}

	return p, nil
}
