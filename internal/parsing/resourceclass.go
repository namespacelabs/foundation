// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"
	"io/fs"

	"google.golang.org/protobuf/reflect/protodesc"
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
			return fnerrors.NewWithLocation(pp.Location, "resource class name can't be empty")
		}

		rc.PackageName = pp.Location.PackageName.String()

		intentType, err := loadUserType(parseOpts, fsys, pp.Location, rc.IntentType)
		if err != nil {
			return err
		}

		instanceType, err := loadUserType(parseOpts, fsys, pp.Location, rc.InstanceType)
		if err != nil {
			return err
		}

		pp.ResourceClasses = append(pp.ResourceClasses, pkggraph.ResourceClass{
			Ref:             &schema.PackageRef{PackageName: rc.PackageName, Name: rc.Name},
			Source:          rc,
			IntentType:      intentType,
			InstanceType:    instanceType,
			DefaultProvider: schema.PackageName(rc.DefaultProvider),
		})
	}

	return nil
}

func loadUserType(parseOpts protos.ParseOpts, fsys fs.FS, loc pkggraph.Location, spec *schema.ResourceClass_Type) (pkggraph.UserType, error) {
	switch spec.ProtoType {
	case "foundation.schema.PackageRef":
		md := (&schema.PackageRef{}).ProtoReflect().Descriptor()
		file := protodesc.ToFileDescriptorProto(md.ParentFile())

		fds := &protos.FileDescriptorSetAndDeps{}
		fds.File = append(fds.File, file)

		files, err := protodesc.NewFiles(fds.AsFileDescriptorSet())
		if err != nil {
			return pkggraph.UserType{}, fnerrors.NewWithLocation(loc, "failed to generate registry files: %v", err)
		}

		return pkggraph.UserType{Descriptor: md, Sources: fds, Registry: files}, nil
	}

	fds, err := parseOpts.ParseAtLocation(fsys, loc, []string{spec.ProtoSource})
	if err != nil {
		return pkggraph.UserType{}, fnerrors.NewWithLocation(loc, "failed to parse proto sources %v: %v", spec.ProtoSource, err)
	}

	files, md, err := protos.LoadMessageByName(fds, spec.ProtoType)
	if err != nil {
		return pkggraph.UserType{}, fnerrors.NewWithLocation(loc, "failed to load message %q: %v", spec.ProtoType, err)
	}

	return pkggraph.UserType{Descriptor: md, Sources: fds, Registry: files}, nil
}
