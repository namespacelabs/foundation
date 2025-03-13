// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"cuelang.org/go/cue"
	"google.golang.org/protobuf/types/descriptorpb"
	"namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueWorkspace struct {
	ServerPath string `json:"serverPath"`
}

type cueServerReference struct {
	PackageName string        `json:"packageName"`
	Id          string        `json:"id"`
	Name        string        `json:"name"`
	Endpoints   []cueEndpoint `json:"endpoints"`
}

type cueEndpoint struct {
	Type               string `json:"type"`
	ServiceName        string `json:"serviceName"`
	AllocatedName      string `json:"allocatedName"`
	FullyQualifiedName string `json:"fullyQualifiedName"`
	ContainerPort      int32  `json:"containerPort"`
}

func FetchServer(packages pkggraph.PackageLoader, stack pkggraph.StackEndpoints) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		var server cueServerReference
		if err := v.Decode(&server); err != nil {
			return nil, err
		}

		pkg, err := packages.LoadByName(ctx, schema.PackageName(server.PackageName))
		if err != nil {
			return nil, err
		}

		if pkg.Server == nil {
			return nil, fnerrors.BadInputError("%s: expected a server", pkg.PackageName())
		}

		server.Id = pkg.Server.Id
		server.Name = pkg.Server.Name
		server.Endpoints = []cueEndpoint{}

		for _, endpoint := range stack.EndpointsBy(pkg.PackageName()) {
			ep := cueEndpoint{
				Type:               endpoint.Type.String(),
				ServiceName:        endpoint.ServiceName,
				AllocatedName:      endpoint.AllocatedName,
				FullyQualifiedName: endpoint.FullyQualifiedName,
			}

			if len(endpoint.GetPorts()) > 0 {
				ep.ContainerPort = endpoint.Ports[0].Port.GetContainerPort()
			}

			server.Endpoints = append(server.Endpoints, ep)
		}

		return server, nil
	}
}

func FetchServerWorkspace(loc protos.Location) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		return cueWorkspace{
			ServerPath: loc.Rel(),
		}, nil
	}
}

type cueProtoload struct {
	Sources     []string `json:"sources"`
	SkipCodegen bool     `json:"skip_codegen"`

	Types    map[string]cueProto `json:"types"`
	Services map[string]cueProto `json:"services"`
}

func FetchProto(pl pkggraph.PackageLoader, fsys fs.FS, loc pkggraph.Location) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		var load cueProtoload
		if err := v.Decode(&load); err != nil {
			return nil, err
		}

		opts, err := parsing.MakeProtoParseOpts(ctx, pl, loc.Module.Workspace)
		if err != nil {
			return nil, err
		}

		fds, err := opts.ParseAtLocation(fsys, loc, load.Sources)
		if err != nil {
			return nil, err
		}

		load.Types = map[string]cueProto{}
		load.Services = map[string]cueProto{}

		for _, d := range fds.File {
			if err := fillFromFile(fds, d, &load, load.SkipCodegen); err != nil {
				return load, err
			}
		}

		return load, nil
	}
}

func fillFromFile(fds *protos.FileDescriptorSetAndDeps, d *descriptorpb.FileDescriptorProto, out *cueProtoload, skipCodegen bool) error {
	for _, index := range d.PublicDependency {
		if int(index) >= len(d.Dependency) {
			return fnerrors.InternalError("%s: public_dependency out of bonds", d.GetName())
		}

		dep := d.Dependency[index]

		var filedep *descriptorpb.FileDescriptorProto
		for _, d := range fds.File {
			if d.GetName() == dep {
				filedep = d
				break
			}
		}
		if filedep == nil {
			for _, d := range fds.Dependency {
				if d.GetName() == dep {
					filedep = d
					break
				}
			}
		}

		if filedep == nil {
			return fnerrors.InternalError("%s: public_dependency refers to unknown dependency %q", d.GetName(), dep)
		}

		if err := fillFromFile(fds, filedep, out, skipCodegen); err != nil {
			return err
		}
	}

	for _, t := range d.GetMessageType() {
		out.Types[t.GetName()] = cueProto{
			Typename:    fmt.Sprintf("%s.%s", d.GetPackage(), t.GetName()),
			Sources:     out.Sources,
			SkipCodegen: skipCodegen,
		}
	}

	for _, svc := range d.GetService() {
		out.Services[svc.GetName()] = cueProto{
			Typename:    fmt.Sprintf("%s.%s", d.GetPackage(), svc.GetName()),
			Sources:     out.Sources,
			SkipCodegen: skipCodegen,
		}
	}

	return nil
}

type cueResource struct {
	Path     string `json:"path"`
	Contents []byte `json:"contents"`
}

func FetchResource(fsys fs.FS, loc pkggraph.Location) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		var load cueResource
		if err := v.Decode(&load); err != nil {
			return nil, err
		}

		if load.Path == "" {
			return nil, fnerrors.NewWithLocation(loc, "#FromPath needs to have a path specified")
		}

		rsc, err := LoadResource(fsys, loc, load.Path)
		if err != nil {
			return nil, err
		}

		load.Contents = rsc.Contents
		return load, nil
	}
}

func LoadResource(fsys fs.FS, loc pkggraph.Location, path string) (*schema.FileContents, error) {
	if strings.HasPrefix(path, "../") {
		return nil, fnerrors.NewWithLocation(loc, "resources must be loaded from within the package")
	}

	contents, err := fs.ReadFile(fsys, loc.Rel(path))
	if err != nil {
		return nil, err
	}

	return &schema.FileContents{
		Path:     loc.Rel(path),
		Contents: contents,
	}, nil
}

func FetchPackage(pl pkggraph.PackageLoader) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		packageName, err := v.String()
		if err != nil {
			return nil, fnerrors.Newf("expected a string when loading a package: %w", err)
		}

		return ConsumeNoValue, pl.Ensure(ctx, schema.PackageName(packageName))
	}
}
