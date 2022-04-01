// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func Register() {
	languages.Register(schema.Framework_NODEJS, impl{})

	ops.Register[*OpGenServer](generator{})
}

type generator struct{}

func (generator) Run(ctx context.Context, env ops.Environment, _ *schema.Definition, msg *OpGenServer) (*ops.DispatcherResult, error) {
	wenv, ok := env.(workspace.Packages)
	if !ok {
		return nil, errors.New("workspace.Packages required")
	}

	loc, err := wenv.Resolve(ctx, schema.PackageName(msg.Server.PackageName))
	if err != nil {
		return nil, err
	}

	return nil, fnfs.WriteWorkspaceFile(ctx, loc.Module.ReadWriteFS(), loc.Rel("main.fn.ts"), func(w io.Writer) error {
		f, err := resources.Open("main.ts")
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(w, f)
		return err
	})
}

type impl struct {
	languages.MaybeGenerate
	languages.MaybeTidy
	languages.NoDev
}

func (impl) PrepareBuild(ctx context.Context, _ languages.Endpoints, server provision.Server, isFocus bool) (build.Spec, error) {
	return buildNodeJS{loc: server.Location, isFocus: isFocus}, nil
}

func (impl) PrepareRun(ctx context.Context, t provision.Server, run *runtime.ServerRunOpts) error {
	run.Command = []string{"node", "main.fn.js"}
	run.WorkingDir = "/app"
	run.ReadOnlyFilesystem = true
	run.RunAs = production.NonRootRunAsWithID(production.NonRootUserID)
	return nil
}

func (impl) TidyServer(ctx context.Context, loc workspace.Location, server *schema.Server) error {
	var packages, devPackages []string

	for pkg, version := range builtin().Dependencies {
		packages = append(packages, fmt.Sprintf("%s@%s", pkg, version))
	}

	for pkg, version := range builtin().DevDependencies {
		devPackages = append(devPackages, fmt.Sprintf("%s@%s", pkg, version))
	}

	sort.Strings(packages)
	sort.Strings(devPackages)

	if len(packages) > 0 {
		if err := RunYarn(ctx, loc, append([]string{"add"}, packages...)); err != nil {
			return err
		}
	}

	if len(devPackages) > 0 {
		if err := RunYarn(ctx, loc, append([]string{"add", "-D"}, devPackages...)); err != nil {
			return err
		}
	}

	return nil
}

func (impl) GenerateServer(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.Definition, error) {
	var dl defs.DefList
	dl.Add("Generate Typescript server dependencies", &OpGenServer{Server: pkg.Server}, pkg.PackageName())
	return dl.Serialize()
}

func (impl) ParseNode(ctx context.Context, loc workspace.Location, ext *workspace.FrameworkExt) error {
	return nil
}

func (impl) PreParseServer(ctx context.Context, loc workspace.Location, ext *workspace.FrameworkExt) error {
	ext.Include = append(ext.Include, "namespacelabs.dev/foundation/std/nodejs/grpc")
	return nil
}

func (impl) PostParseServer(ctx context.Context, _ *workspace.Sealed) error {
	return nil
}

func (impl) InjectService(loc workspace.Location, node *schema.Node, svc *workspace.CueService) error {
	return nil
}

func (impl) GenerateNode(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.Definition, error) {
	var dl defs.DefList

	var list []*protos.FileDescriptorSetAndDeps
	for _, dl := range pkg.Provides {
		list = append(list, dl)
	}
	for _, svc := range pkg.Services {
		list = append(list, svc)
	}

	if len(list) > 0 {
		dl.Add("Generate Javascript/Typescript proto sources", &source.OpProtoGen{
			PackageName:         pkg.PackageName().String(),
			GenerateHttpGateway: pkg.Node().ExportServicesAsHttp,
			Protos:              protos.Merge(list...),
			Framework:           source.OpProtoGen_TYPESCRIPT,
		})
	}

	return dl.Serialize()
}
