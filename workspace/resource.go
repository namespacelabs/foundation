// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/source/protos"
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

	for _, rc := range pp.ResourceClasses {
		if rc.Name == "" {
			return fnerrors.UserError(pp.Location, "resource class name can't be empty")
		}

		rc.PackageName = pp.Location.PackageName.String()

		fds, err := transformResourceClass(parseOpts, fsys, pp.Location, []*schema.ResourceClass_Type{rc.IntentType, rc.InstanceType})
		if err != nil {
			return err
		}

		if pp.Provides == nil {
			pp.Provides = map[string]*protos.FileDescriptorSetAndDeps{}
		}
		pp.Provides[rc.Name] = fds
	}

	return nil
}

func transformResourceClass(parseOpts protos.ParseOpts, fsys fs.FS, loc pkggraph.Location, rcTypes []*schema.ResourceClass_Type) (*protos.FileDescriptorSetAndDeps, error) {
	protoSources := []string{}
	for _, rcType := range rcTypes {
		protoSources = append(protoSources, rcType.ProtoSource)
	}

	fds, err := parseOpts.ParseAtLocation(fsys, loc, protoSources)
	if err != nil {
		return nil, fnerrors.UserError(loc, "failed to parse proto sources %v: %v", protoSources, err)
	}

	for _, t := range rcTypes {
		if _, _, err := protos.LoadMessageByName(fds, t.ProtoType); err != nil {
			return nil, fnerrors.UserError(loc, "failed to load message %q: %v", t.ProtoType, err)
		}
	}

	return fds, nil
}
