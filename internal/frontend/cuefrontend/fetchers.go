// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"google.golang.org/protobuf/types/descriptorpb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

type cueWorkspace struct {
	ModuleName string `json:"moduleName"`
	ServerPath string `json:"serverPath"`
}

type cueServiceRequest struct {
	PackageName string `json:"packageName"`
}

type cueServerReference struct {
	PackageName string        `json:"packageName"`
	Id          string        `json:"id"`
	Name        string        `json:"name"`
	Endpoints   []cueEndpoint `json:"endpoints"`
}

type cueEndpoint struct {
	Type          string `json:"type"`
	ServiceName   string `json:"serviceName"`
	AllocatedName string `json:"allocatedName"`
	ContainerPort int32  `json:"containerPort"`
}

func FetchService(pl workspace.Packages) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		var req cueServiceRequest
		if err := v.Decode(&req); err != nil {
			return nil, err
		}

		pkg, err := pl.LoadByName(ctx, schema.PackageName(req.PackageName))
		if err != nil {
			return nil, err
		}

		if pkg.Service == nil {
			return nil, fnerrors.UserError(pkg.Location, "%v: expected schema to contain a single service", req.PackageName)
		}

		svc := pkg.Service

		if len(svc.ExportService) != 1 {
			return nil, fnerrors.UserError(nil, "#input.Service can only be used on nodes which export exactly one service, saw %d", len(svc.ExportService))
		}

		// XXX use protoreflect.FullName(svc.ExportService[0].ProtoTypename).Name()
		peer := workspace.CueService{
			ProtoTypename: simpleName(svc.ExportService[0].ProtoTypename),
		}

		for _, handler := range workspace.FrameworkHandlers {
			if err := handler.InjectService(pkg.Location, svc, &peer); err != nil {
				return nil, err
			}
		}

		return peer, nil
	}
}

func simpleName(serviceProtoType string) string {
	parts := strings.Split(serviceProtoType, ".")
	return parts[len(parts)-1]
}

func FetchServer(packages workspace.Packages, stack *schema.Stack) FetcherFunc {
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

		s := stack.GetServer(pkg.PackageName())
		if s != nil {
			for _, endpoint := range stack.EndpointsBy(pkg.PackageName()) {
				server.Endpoints = append(server.Endpoints, cueEndpoint{
					Type:          endpoint.Type.String(),
					ServiceName:   endpoint.ServiceName,
					AllocatedName: endpoint.AllocatedName,
					ContainerPort: endpoint.GetPort().GetContainerPort(),
				})
			}
		}

		return server, nil
	}
}

func FetchServerWorkspace(workspace *schema.Workspace, loc protos.Location) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		return cueWorkspace{
			ModuleName: workspace.ModuleName,
			ServerPath: loc.Rel(),
		}, nil
	}
}

type cueProtoload struct {
	Sources []string `json:"sources"`

	Types    map[string]cueProto `json:"types"`
	Services map[string]cueProto `json:"services"`
}

type cueProto struct {
	Typename string   `json:"typename"`
	Source   []string `json:"source"`
}

func FetchProto(fsys fs.FS, loc workspace.Location) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		var load cueProtoload
		if err := v.Decode(&load); err != nil {
			return nil, err
		}

		fds, err := protos.ParseAtLocation(fsys, loc, load.Sources)
		if err != nil {
			return nil, err
		}

		load.Types = map[string]cueProto{}
		load.Services = map[string]cueProto{}

		for _, d := range fds.File {
			if err := fillFromFile(fds, d, &load); err != nil {
				return load, err
			}
		}

		return load, nil
	}
}

func fillFromFile(fds *protos.FileDescriptorSetAndDeps, d *descriptorpb.FileDescriptorProto, out *cueProtoload) error {
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

		if err := fillFromFile(fds, filedep, out); err != nil {
			return err
		}
	}

	for _, t := range d.GetMessageType() {
		out.Types[t.GetName()] = cueProto{
			Typename: fmt.Sprintf("%s.%s", d.GetPackage(), t.GetName()),
			Source:   out.Sources,
		}
	}

	for _, svc := range d.GetService() {
		out.Services[svc.GetName()] = cueProto{
			Typename: fmt.Sprintf("%s.%s", d.GetPackage(), svc.GetName()),
			Source:   out.Sources,
		}
	}

	return nil
}

type cueResource struct {
	Path     string `json:"path"`
	Contents []byte `json:"contents"`
}

func FetchResource(fsys fs.FS, loc workspace.Location) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		var load cueResource
		if err := v.Decode(&load); err != nil {
			return nil, err
		}

		if load.Path == "" {
			return nil, fnerrors.UserError(loc, "#FromPath needs to have a path specified")
		}

		if strings.HasPrefix(load.Path, "../") {
			return nil, fnerrors.UserError(loc, "#FromPath path must be relative to the node")
		}

		contents, err := fs.ReadFile(fsys, loc.Rel(load.Path))
		if err != nil {
			return nil, err
		}

		load.Contents = contents
		return load, nil
	}
}

func FetchPackage(pl workspace.Packages) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		packageName, err := v.String()
		if err != nil {
			return nil, fnerrors.UserError(nil, "expected a string when loading a package: %w", err)
		}

		_, err = pl.LoadByName(ctx, schema.PackageName(packageName))
		return ConsumeNoValue, err
	}
}

type cueServerResult struct {
	Image     string `json:"image"`
	Framework string `json:"framework"`
}

func FetchFocusServer(serverImageRef string, srv *schema.Server) FetcherFunc {
	return func(c context.Context, v cue.Value) (interface{}, error) {
		return cueServerResult{Image: serverImageRef, Framework: srv.Framework.String()}, nil
	}
}

func FetchEnv(env *schema.Environment, workspace *schema.Workspace) FetcherFunc {
	return func(context.Context, cue.Value) (interface{}, error) {
		return cueEnv{Name: env.Name, Runtime: env.Runtime, Purpose: env.Purpose.String(), Ephemeral: env.Ephemeral}, nil
	}
}

type cueEnv struct {
	Name      string `json:"name"`
	Runtime   string `json:"runtime"`
	Purpose   string `json:"purpose"`
	Ephemeral bool   `json:"ephemeral"`
}

func FetchVCS(rootDir string) FetcherFunc {
	return func(ctx context.Context, v cue.Value) (interface{}, error) {
		status, err := git.FetchStatus(ctx, rootDir)
		if err != nil {
			return nil, err
		}

		return cueVCS{Revision: status.Revision, CommitTime: status.CommitTime, Uncommitted: status.Uncommitted}, nil
	}
}

type cueVCS struct {
	Revision    string    `json:"revision"`
	CommitTime  time.Time `json:"commitTime"`
	Uncommitted bool      `json:"uncommitted"`
}
