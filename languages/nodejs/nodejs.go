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
	"io/fs"
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
const yarnVersion = "3.2.0"
const yarnRcFn = ".yarnrc.yml"
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

func (impl) TidyWorkspace(ctx context.Context, packages []*workspace.Package) error {
	yarnRoots := map[string]*workspace.Module{}
	for _, pkg := range packages {
		if (pkg.Server != nil && pkg.Server.Framework == schema.Framework_NODEJS) ||
			(pkg.Node() != nil && pkg.Node().ServiceFramework == schema.Framework_NODEJS) {
			yarnRoot, err := findYarnRoot(pkg.Location)
			if err != nil {
				// If we can't find yarn root, using the workspace root.
				yarnRoot = ""
			}
			// It is always the same module, but saving it as a value allows to avoid additional checks for nil.
			yarnRoots[yarnRoot.String()] = pkg.Location.Module
		}
	}

	for yarnRoot, module := range yarnRoots {
		tidyYarnRoot(ctx, yarnRoot, module)
	}

	return nil
}

func tidyYarnRoot(ctx context.Context, path string, module *workspace.Module) error {
	installYarn := false
	_, err := updatePackageJson(ctx, path, module.ReadWriteFS(), func(packageJson map[string]interface{}, fileExisted bool) {
		packageJson["private"] = true
		packageJson["workspaces"] = []string{"**/*"}
		yarnWithVersion := fmt.Sprintf("yarn@%s", yarnVersion)
		installYarn = packageJson["packageManager"] != yarnWithVersion
	})

	if err != nil {
		return err
	}

	// Install Yarn 3+ if needed
	if installYarn {
		if err := RunYarn(ctx, path, []string{"set", "version", yarnVersion}); err != nil {
			return err
		}
	}

	if err := fnfs.WriteWorkspaceFile(ctx, module.ReadWriteFS(), filepath.Join(path, yarnRcFn), func(w io.Writer) error {
		_, err := io.WriteString(w, yarnRcContent())
		return err
	}); err != nil {
		return err
	}

	tsconfigFn := filepath.Join(path, "tsconfig.json")
	if _, err := updateJson(ctx, tsconfigFn, module.ReadWriteFS(),
		func(packageJson map[string]interface{}, fileExisted bool) {
			if !fileExisted {
				packageJson["extends"] = "@tsconfig/node16/tsconfig.json"
			}
		}); err != nil {
		return err
	}

	return nil
}

func yarnRcContent() string {
	return fmt.Sprintf(
		`nodeLinker: node-modules

yarnPath: .yarn/releases/yarn-%s.cjs
`, yarnVersion)
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

	_, err := fs.Stat(p.Location.Module.ReadWriteFS(), implFn)
	if err == nil || !os.IsNotExist(err) {
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
	packageJson, err := tidyPackageJsonFields(ctx, loc)
	if err != nil {
		return err
	}

	wasYarnCalled, err := tidyDependencies(ctx, loc, imports, packageJson)
	if err != nil {
		return err
	}

	if wasYarnCalled {
		// If yarn messed with package.json, re-format it to make tn tidy idempotent.
		_, err = tidyPackageJsonFields(ctx, loc)
		return err
	} else {
		return nil
	}
}

// Returns the tydied package.json file.
func tidyPackageJsonFields(ctx context.Context, loc workspace.Location) (map[string]interface{}, error) {
	nodejsLoc, err := nodejsLocationFrom(loc.PackageName)
	if err != nil {
		return nil, err
	}

	return updatePackageJson(ctx, loc.Rel(), loc.Module.ReadWriteFS(), func(packageJson map[string]interface{}, fileExisted bool) {
		packageJson["name"] = nodejsLoc.NpmPackage
		packageJson["private"] = true
		packageJson["version"] = yarnWorkspaceVersion
	})
}

func updatePackageJson(ctx context.Context, path string, fs fnfs.ReadWriteFS, callback func(json map[string]interface{}, fileExisted bool)) (map[string]interface{}, error) {
	return updateJson(ctx, filepath.Join(path, packageJsonFn), fs, callback)
}

func updateJson(ctx context.Context, filepath string, fs fnfs.ReadWriteFS, callback func(json map[string]interface{}, fileExisted bool)) (map[string]interface{}, error) {
	parsedJson := map[string]interface{}{}

	jsonFile, err := fs.Open(filepath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	fileExisted := err == nil
	if err == nil {
		defer jsonFile.Close()

		jsonRaw, err := io.ReadAll(jsonFile)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(jsonRaw, &parsedJson)
	}

	callback(parsedJson, fileExisted)

	updatedJsonRaw, err := json.MarshalIndent(parsedJson, "", "\t")
	if err != nil {
		return nil, err
	}

	return parsedJson, fnfs.WriteWorkspaceFile(ctx, fs, filepath, func(w io.Writer) error {
		_, err := w.Write(updatedJsonRaw)
		return err
	})
}

// Returns true if yarn was called.
func tidyDependencies(ctx context.Context, loc workspace.Location, imports []string, packageJson map[string]interface{}) (bool, error) {
	dependencies := map[string]string{}
	for key, value := range builtin().Dependencies {
		dependencies[key] = value
	}
	for _, importName := range imports {
		loc, err := nodejsLocationFrom(schema.Name(importName))
		if err != nil {
			return false, err
		}
		dependencies[loc.NpmPackage] = yarnWorkspaceVersion
	}

	packages := formatPackages(trimExistingPackages(dependencies, packageJson["dependencies"]))
	devPackages := formatPackages(trimExistingPackages(builtin().DevDependencies, packageJson["devDependencies"]))

	if len(packages) > 0 {
		if err := RunYarn(ctx, loc.Rel(), append([]string{"add"}, packages...)); err != nil {
			return false, err
		}
	}

	if len(devPackages) > 0 {
		if err := RunYarn(ctx, loc.Rel(), append([]string{"add", "-D"}, devPackages...)); err != nil {
			return false, err
		}
	}

	return len(packages) > 0 || len(devPackages) > 0, nil
}

func trimExistingPackages(packages map[string]string, existingPackages interface{}) map[string]string {
	if existingPackages == nil {
		return packages
	}

	existingPackagesMap := existingPackages.(map[string]interface{})

	trimmedPackages := map[string]string{}
	for pkgName, version := range packages {
		if existingVersion, ok := existingPackagesMap[pkgName]; !ok || version != existingVersion {
			trimmedPackages[pkgName] = version
		}
	}

	return trimmedPackages
}

func formatPackages(packages map[string]string) []string {
	formattedPackages := []string{}
	for pkg, version := range packages {
		formattedPackages = append(formattedPackages, fmt.Sprintf("%s@%s", pkg, version))
	}
	sort.Strings(formattedPackages)

	return formattedPackages
}

func (impl) GenerateServer(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.Definition, error) {
	var dl defs.DefList
	dl.Add("Generate Typescript server dependencies", &OpGenServer{Server: pkg.Server, LoadedNode: nodes}, pkg.PackageName())
	return dl.Serialize()
}

func (impl) ParseNode(ctx context.Context, loc workspace.Location, _ *schema.Node, ext *workspace.FrameworkExt) error {
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
