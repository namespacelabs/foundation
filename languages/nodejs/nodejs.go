// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

// Hard-coding the version of generated yarn workspaces since we only support a monorepo for now.
const yarnWorkspaceVersion = "0.0.0"
const implFileName = "impl.ts"
const packageJsonFn = "package.json"

func Register() {
	languages.Register(schema.Framework_NODEJS, impl{})

	ops.Register[*OpGenServer](generator{})

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, _ *schema.Definition, x *OpGenNode) (*ops.DispatcherResult, error) {
		wenv, ok := env.(workspace.Packages)
		if !ok {
			return nil, errors.New("workspace.Packages required")
		}

		loc, err := wenv.Resolve(ctx, schema.PackageName(x.Node.PackageName))
		if err != nil {
			return nil, err
		}

		return nil, generateNode(ctx, wenv, loc, x.Node, x.LoadedNode, loc.Module.ReadWriteFS())
	})
}

type generator struct{}

func (generator) Run(ctx context.Context, env ops.Environment, _ *schema.Definition, msg *OpGenServer) (*ops.DispatcherResult, error) {
	workspacePackages, ok := env.(workspace.Packages)
	if !ok {
		return nil, errors.New("workspace.Packages required")
	}

	loc, err := workspacePackages.Resolve(ctx, schema.PackageName(msg.Server.PackageName))
	if err != nil {
		return nil, err
	}

	return nil, generateServer(ctx, workspacePackages, loc, msg.Server, msg.LoadedNode, loc.Module.ReadWriteFS())
}

type impl struct {
	languages.MaybeGenerate
	languages.MaybeTidy
	languages.NoDev
}

func (impl) PrepareBuild(ctx context.Context, _ languages.Endpoints, server provision.Server, isFocus bool) (build.Spec, error) {
	deps := []workspace.Location{}
	for _, dep := range server.Deps() {
		if dep.Node() != nil && dep.Node().ServiceFramework == schema.Framework_NODEJS {
			deps = append(deps, dep.Location)
		}
	}

	yarnRoot, err := findYarnRoot(server.Location)
	if err != nil {
		return nil, err
	}

	return buildNodeJS{
		serverLoc: server.Location,
		deps:      deps,
		yarnRoot:  yarnRoot,
		isFocus:   isFocus}, nil
}

func (impl) PrepareRun(ctx context.Context, t provision.Server, run *runtime.ServerRunOpts) error {
	run.Command = []string{"node", filepath.Join(t.Location.Rel(), "main.fn.js")}
	run.WorkingDir = "/app"
	run.ReadOnlyFilesystem = true
	run.RunAs = production.NonRootRunAsWithID(production.NonRootUserID)
	return nil
}

func (impl) TidyNode(ctx context.Context, p *workspace.Package) error {
	err := tidyPackageJson(ctx, p.Location, p.Node().Import)
	if err != nil {
		return err
	}

	return maybeGenerateImplStub(ctx, p)
}

func maybeGenerateImplStub(ctx context.Context, p *workspace.Package) error {
	if len(p.Services) == 0 {
		// This is not an error, the user might have not added anything yet.
		return nil
	}

	implFn := filepath.Join(p.Location.Rel(), implFileName)
	if _, err := os.Stat(implFn); err == nil {
		// File alreasy exists, do nothing
		return nil
	}

	tmplOptions := nodeimplTmplOptions{}
	for key, srv := range p.Services {
		srvNameParts := strings.Split(key, ".")
		srvName := srvNameParts[len(srvNameParts)-1]
		tmplOptions.ServiceServerName = fmt.Sprintf("I%sServer", srvName)
		tmplOptions.ServiceName = fmt.Sprintf("%sService", srvName)

		srvFullFn, err := fileNameForService(srvName, srv.File)
		if err != nil {
			return err
		}
		tmplOptions.ServiceFileName = strings.TrimSuffix(filepath.Base(srvFullFn), filepath.Ext(srvFullFn))

		// Only supporting one service for now.
		break
	}

	return generateSource(ctx, p.Location.Module.ReadWriteFS(), implFn, nodeimplTmpl, tmplOptions)
}

func fileNameForService(srvName string, descriptors []*descriptorpb.FileDescriptorProto) (string, error) {
	for _, file := range descriptors {
		for _, service := range file.Service {
			if *service.Name == srvName {
				return file.GetName(), nil
			}
		}
	}
	return "", fnerrors.InternalError("Couldn't find service %s in the generated proto descriptors.", srvName)
}

func (impl) TidyServer(ctx context.Context, loc workspace.Location, server *schema.Server) error {
	return tidyPackageJson(ctx, loc, server.Import)
}

func tidyPackageJson(ctx context.Context, loc workspace.Location, imports []string) error {
	// If package.json doesn't exist, creating it before calling yarn.
	// We always need to call "tidyPackageJsonFields" again in the end to keep
	// the field order alphabetical to make "tidy" idempotent.
	// yarn messes with package.json in a different way: adds new fields to the end
	// but also removes empty fields like "dependencies".
	err := tidyPackageJsonFields(ctx, loc)
	if err != nil {
		return err
	}

	err = tidyDependencies(ctx, loc, imports)
	if err != nil {
		return err
	}

	return tidyPackageJsonFields(ctx, loc)
}

func tidyPackageJsonFields(ctx context.Context, loc workspace.Location) error {
	packageJson := map[string]interface{}{}

	packageJsonRelFn := filepath.Join(loc.Rel(), packageJsonFn)
	packageJsonFile, err := loc.Module.ReadWriteFS().Open(packageJsonRelFn)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		defer packageJsonFile.Close()

		packageJsonRaw, err := io.ReadAll(packageJsonFile)
		if err != nil {
			return err
		}

		json.Unmarshal(packageJsonRaw, &packageJson)
	}

	nodejsLoc, err := nodejsLocationFrom(loc.PackageName)
	if err != nil {
		return err
	}
	packageJson["name"] = nodejsLoc.NpmPackage
	packageJson["private"] = true
	packageJson["version"] = yarnWorkspaceVersion

	editedPackageJsonRaw, err := json.MarshalIndent(packageJson, "", "\t")
	if err != nil {
		return err
	}

	return fnfs.WriteWorkspaceFile(ctx, loc.Module.ReadWriteFS(), packageJsonRelFn, func(w io.Writer) error {
		_, err := w.Write(editedPackageJsonRaw)
		return err
	})
}

func tidyDependencies(ctx context.Context, loc workspace.Location, imports []string) error {
	var packages, devPackages []string

	for pkg, version := range builtin().Dependencies {
		packages = append(packages, fmt.Sprintf("%s@%s", pkg, version))
	}

	for _, importName := range imports {
		loc, err := nodejsLocationFrom(schema.Name(importName))
		if err != nil {
			return err
		}
		packages = append(packages, fmt.Sprintf("%s@%s", loc.NpmPackage, yarnWorkspaceVersion))
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
	dl.Add("Generate Typescript server dependencies", &OpGenServer{Server: pkg.Server, LoadedNode: nodes}, pkg.PackageName())
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

func (impl) EvalProvision(*schema.Node) (frontend.ProvisionStack, error) {
	return frontend.ProvisionStack{}, nil
}

func (impl) GenerateNode(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.Definition, error) {
	var dl defs.DefList

	dl.Add("Generate Nodejs node dependencies", &OpGenNode{
		Node:       pkg.Node(),
		LoadedNode: nodes,
	}, pkg.PackageName())

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
