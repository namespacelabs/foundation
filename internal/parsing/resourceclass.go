// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package parsing

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func transformResourceClasses(ctx context.Context, pl EarlyPackageLoader, pp *pkggraph.Package) error {
	parseOpts, err := MakeProtoParseOpts(ctx, pl, pp.Location.Module.Workspace)
	if err != nil {
		return err
	}

	fsys, err := pl.WorkspaceOf(ctx, pp.Location.Module)
	if err != nil {
		return err
	}

	for _, rc := range pp.ResourceClassSpecs {
		if rc.Name == "" {
			return fnerrors.UserError(pp.Location, "resource class name can't be empty")
		}

		rc.PackageName = pp.Location.PackageName.String()

		intentType, err := loadUserType(parseOpts, fsys, pp.Location, rc.IntentType)
		if err != nil {
			return err
		}

		InstanceType, err := loadUserType(parseOpts, fsys, pp.Location, rc.InstanceType)
		if err != nil {
			return err
		}

		pp.ResourceClasses = append(pp.ResourceClasses, pkggraph.ResourceClass{
			Ref:          &schema.PackageRef{PackageName: rc.PackageName, Name: rc.Name},
			Source:       rc,
			IntentType:   intentType,
			InstanceType: InstanceType,
		})
	}

	return nil
}

func loadUserType(parseOpts protos.ParseOpts, fsys fs.FS, loc pkggraph.Location, spec *schema.ResourceClass_Type) (pkggraph.UserType, error) {
	fds, err := parseOpts.ParseAtLocation(fsys, loc, []string{spec.ProtoSource})
	if err != nil {
		return pkggraph.UserType{}, fnerrors.UserError(loc, "failed to parse proto sources %v: %v", spec.ProtoSource, err)
	}

	files, md, err := protos.LoadMessageByName(fds, spec.ProtoType)
	if err != nil {
		return pkggraph.UserType{}, fnerrors.UserError(loc, "failed to load message %q: %v", spec.ProtoType, err)
	}

	return pkggraph.UserType{Descriptor: md, Sources: fds, Registry: files}, nil
}
