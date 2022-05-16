// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

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

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/descriptorpb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/internal/yarn"
	"namespacelabs.dev/foundation/languages"
	nodejsruntime "namespacelabs.dev/foundation/languages/nodejs/runtime"
	"namespacelabs.dev/foundation/languages/nodejs/yarnplugin"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

const (
	controllerPkg schema.PackageName = "namespacelabs.dev/foundation/std/development/filesync/controller"
	grpcPkg       schema.PackageName = "namespacelabs.dev/foundation/languages/nodejs/grpc"
	// Yarn version of the packages in the same module. Doesn't really matter what the value here is.
	defaultPackageVersion = "0.0.0"
	yarnRcFn              = ".yarnrc.yml"
	fnYarnPluginPath      = ".yarn/plugins/plugin-foundation.cjs"
	yarnGitIgnore         = ".yarn/.gitignore"
	implFileName          = "impl.ts"
	packageJsonFn         = "package.json"
	fileSyncPort          = 50000
	runtimePackage        = "@namespacelabs/foundation"
	ForceProd             = false
)

var (
	yarnRcContent = fmt.Sprintf(
		`nodeLinker: node-modules

npmScopes:
  namespacelabs:
    npmRegistryServer: "https://us-npm.pkg.dev/foundation-344819/npm-prebuilts/"

plugins:
  - path: %s
`, fnYarnPluginPath)
	yarnGitIgnoreContent = fmt.Sprintf(
		`/.gitignore
%s
`, strings.TrimPrefix(fnYarnPluginPath, ".yarn"))
)

func Register() {
	languages.Register(schema.Framework_NODEJS, impl{})

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, _ *schema.Definition, x *OpGenServer) (*ops.HandleResult, error) {
		workspacePackages, ok := env.(workspace.Packages)
		if !ok {
			return nil, errors.New("workspace.Packages required")
		}

		loc, err := workspacePackages.Resolve(ctx, schema.PackageName(x.Server.PackageName))
		if err != nil {
			return nil, err
		}

		if err := generateServer(ctx, workspacePackages, loc, x.Server, x.LoadedNode, loc.Module.ReadWriteFS()); err != nil {
			return nil, fnerrors.InternalError("failed to generate server: %w", err)
		}

		return nil, nil
	})

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, _ *schema.Definition, x *OpGenNode) (*ops.HandleResult, error) {
		wenv, ok := env.(workspace.Packages)
		if !ok {
			return nil, fnerrors.New("workspace.Packages required")
		}

		loc, err := wenv.Resolve(ctx, schema.PackageName(x.Node.PackageName))
		if err != nil {
			return nil, err
		}

		return nil, generateNode(ctx, wenv, loc, x.Node, x.LoadedNode, loc.Module.ReadWriteFS())
	})

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, _ *schema.Definition, x *OpGenNodeStub) (*ops.HandleResult, error) {
		wenv, ok := env.(workspace.Packages)
		if !ok {
			return nil, fnerrors.New("workspace.Packages required")
		}

		pkg, err := wenv.LoadByName(ctx, schema.PackageName(x.Node.PackageName))
		if err != nil {
			return nil, err
		}

		return nil, generateNodeImplStub(ctx, pkg, x.Filename, x.Node)
	})

	ops.Register[*OpGenYarnRoot](yarnRootStatefulGen{})
}

func useDevBuild(env *schema.Environment) bool {
	// TODO(@nicolasalt): uncomment when #313 is fixed.
	// Currently only dev way to pass flags to the server works, see PostParseServer.
	return !ForceProd && env.Purpose == schema.Environment_DEVELOPMENT
}

type impl struct {
	languages.MaybeGenerate
	languages.MaybeTidy
	languages.NoDev
}

func (impl) PrepareBuild(ctx context.Context, _ languages.Endpoints, server provision.Server, isFocus bool) (build.Spec, error) {
	deps := []workspace.Location{}
	for _, dep := range server.Deps() {
		if dep.Node() != nil && slices.Contains(dep.Node().CodegeneratedFrameworks(), schema.Framework_NODEJS) {
			deps = append(deps, dep.Location)
		}
	}

	yarnRoot, err := findYarnRoot(server.Location)
	if err != nil {
		return nil, err
	}

	locs := append(deps, server.Location)

	isDevBuild := useDevBuild(server.Env().Proto())

	var module build.Workspace
	if r := wsremote.Ctx(ctx); r != nil && isFocus && !server.Location.Module.IsExternal() && isDevBuild {
		module = yarn.YarnHotReloadModule{
			Mod: server.Location.Module,
			// "ModuleName" is empty because we have only one module in the image and
			// we can put everything under the root "/app" directory.
			Sink: r.For(&wsremote.Signature{ModuleName: "", Rel: yarnRoot}),
		}
	} else {
		module = server.Location.Module
	}

	return buildNodeJS{
		module:     module,
		locs:       locs,
		yarnRoot:   yarnRoot,
		serverEnv:  server.Env(),
		isDevBuild: isDevBuild,
		isFocus:    isFocus,
	}, nil
}

func (impl) PrepareDev(ctx context.Context, srv provision.Server) (context.Context, languages.DevObserver, error) {
	if useDevBuild(srv.Env().Proto()) {
		if wsremote.Ctx(ctx) != nil {
			return nil, nil, fnerrors.UserError(srv.Location, "`fn dev` on multiple web/nodejs servers not supported")
		}

		devObserver := hotreload.NewFileSyncDevObserver(ctx, srv, fileSyncPort)

		newCtx, _ := wsremote.WithRegistrar(ctx, devObserver.Deposit)

		return newCtx, devObserver, nil
	}

	return ctx, nil, nil
}

func (impl) PrepareRun(ctx context.Context, srv provision.Server, run *runtime.ServerRunOpts) error {
	if useDevBuild(srv.Env().Proto()) {
		// For dev builds we use runtime complication of Typescript.
		run.ReadOnlyFilesystem = false

		run.Command = []string{"/filesync-controller"}
		run.Args = []string{"/app", fmt.Sprint(fileSyncPort), "nodemon",
			"--config", nodemonConfigPath,
			filepath.Join(srv.Location.Rel(), "main.fn.ts")}
	} else {
		run.Command = []string{"node", filepath.Join(srv.Location.Rel(), "main.fn.js")}
		run.ReadOnlyFilesystem = true
		// See internal/production/images.go.
		fsGroup := production.DefaultFSGroup
		run.RunAs = production.NonRootRunAsWithID(production.DefaultNonRootUserID, &fsGroup)
	}
	run.WorkingDir = "/app"
	return nil
}

type yarnRootData struct {
	moduleFs       fnfs.ReadWriteFS
	workspacePaths []string
}

func (impl) TidyWorkspace(ctx context.Context, packages []*workspace.Package) error {
	yarnRootsMap := map[string]*yarnRootData{}
	yarnRoots := []string{}
	for _, pkg := range packages {
		if (pkg.Server != nil && pkg.Server.Framework == schema.Framework_NODEJS) ||
			(pkg.Node() != nil && slices.Contains(pkg.Node().CodegeneratedFrameworks(), schema.Framework_NODEJS)) {
			yarnRoot, err := findYarnRoot(pkg.Location)
			if err != nil {
				// If we can't find yarn root, using the workspace root.
				yarnRoot = ""
			}
			if yarnRootsMap[yarnRoot] == nil {
				yarnRootsMap[yarnRoot] = &yarnRootData{
					moduleFs:       pkg.Location.Module.ReadWriteFS(),
					workspacePaths: []string{},
				}
				yarnRoots = append(yarnRoots, yarnRoot)
			}
			yarnRootData := yarnRootsMap[yarnRoot]

			relpath, err := filepath.Rel(yarnRoot, pkg.Location.Rel())
			if err != nil {
				return err
			}
			yarnRootData.workspacePaths = append(yarnRootData.workspacePaths, relpath)
		}
	}

	// Iterating over a list for the stable order.
	for _, yarnRoot := range yarnRoots {
		if err := updateYarnRootPackageJson(ctx, yarnRootsMap[yarnRoot], yarnRoot); err != nil {
			return err
		}
		// `fn tidy` could update dependencies of some nodes/servers, running `yarn install` to update
		// `node_modules`.
		if err := yarn.RunYarn(ctx, yarnRoot, []string{"install"}); err != nil {
			return err
		}
	}

	return nil
}

func updateYarnRootPackageJson(ctx context.Context, yarnRootData *yarnRootData, path string) error {
	_, err := updatePackageJson(ctx, path, yarnRootData.moduleFs, func(packageJson map[string]interface{}, fileExisted bool) {
		packageJson["private"] = true
		packageJson["workspaces"] = yarnRootData.workspacePaths
		packageJson["devDependencies"] = map[string]string{
			"typescript": builtin().Dependencies["typescript"],
		}
	})

	return err
}

func (impl) TidyNode(ctx context.Context, pkgs workspace.Packages, p *workspace.Package) error {
	depPkgNames := []string{}
	for _, ref := range p.Node().Reference {
		if ref.PackageName != "" {
			depPkgNames = append(depPkgNames, ref.PackageName)
		}
	}

	return tidyPackageJson(ctx, pkgs, p.Location, depPkgNames)
}

func maybeGenerateNodeImplStub(pkg *workspace.Package, dl *defs.DefList) {
	if len(pkg.Services) == 0 {
		// This is not an error, the user might have not added anything yet.
		return
	}

	implFn := filepath.Join(pkg.Location.Rel(), implFileName)

	_, err := fs.Stat(pkg.Location.Module.ReadWriteFS(), implFn)
	if err == nil || !os.IsNotExist(err) {
		// File already exists, do nothing
		return
	}

	dl.Add("Generate Nodejs node stub", &OpGenNodeStub{
		Node:     pkg.Node(),
		Filename: implFn,
	}, pkg.PackageName())
}

func generateNodeImplStub(ctx context.Context, pkg *workspace.Package, filename string, n *schema.Node) error {
	tmplOptions := nodeImplTmplOptions{}
	for key, srv := range pkg.Services {
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

	return generateSource(ctx, pkg.Location.Module.ReadWriteFS(), filename, tmpl, "Node stub", tmplOptions)
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

func (impl) TidyServer(ctx context.Context, pkgs workspace.Packages, loc workspace.Location, server *schema.Server) error {
	return tidyPackageJson(ctx, pkgs, loc, server.Import)
}

func tidyPackageJson(ctx context.Context, pkgs workspace.Packages, loc workspace.Location, depPkgNames []string) error {
	npmPackage, err := toNpmPackage(loc)
	if err != nil {
		return err
	}

	dependencies := map[string]string{
		runtimePackage: nodejsruntime.RuntimeVersion(),
	}
	for key, value := range builtin().Dependencies {
		dependencies[key] = value
	}
	for _, importName := range depPkgNames {
		pkg, err := pkgs.LoadByName(ctx, schema.PackageName(importName))
		if err != nil {
			return err
		}

		if pkg.Node() != nil && slices.Contains(pkg.Node().CodegeneratedFrameworks(), schema.Framework_NODEJS) {
			importNpmPackage, err := toNpmPackage(pkg.Location)
			if err != nil {
				return err
			}

			var ref string
			if pkg.Location.Module.IsExternal() {
				ref = "fn:" + pkg.Location.PathInCache()
			} else {
				ref = defaultPackageVersion
			}

			dependencies[string(importNpmPackage)] = ref
		}
	}

	_, err = updatePackageJson(ctx, loc.Rel(), loc.Module.ReadWriteFS(), func(packageJson map[string]interface{}, fileExisted bool) {
		packageJson["name"] = npmPackage
		packageJson["private"] = true
		packageJson["version"] = defaultPackageVersion

		packageJson["dependencies"] = mergeJsonMap(packageJson["dependencies"], dependencies)
		packageJson["devDependencies"] = mergeJsonMap(packageJson["devDependencies"], builtin().DevDependencies)
	})

	return err
}

func mergeJsonMap(existingValues interface{}, newValues map[string]string) map[string]string {
	if existingValues == nil {
		return newValues
	}

	existingValueMap, ok := existingValues.(map[string]interface{})
	if !ok {
		existingValueMap = map[string]interface{}{}
	}

	resultMap := map[string]string{}
	for key, value := range existingValueMap {
		resultMap[key] = fmt.Sprintf("%s", value)
	}
	for key, value := range newValues {
		resultMap[key] = value
	}

	return resultMap
}

func updatePackageJson(ctx context.Context, path string, fsys fnfs.ReadWriteFS, callback func(json map[string]interface{}, fileExisted bool)) (map[string]interface{}, error) {
	// We are not using a struct to parse package.json because:
	//  - it may be customized with non-standard keys.
	//  - some keys (for example, "workspaces", even though this particular key the user shouldn't set),
	//    may be an array or an object, and this can't be represented with a struct.
	return updateJson(ctx, filepath.Join(path, packageJsonFn), fsys, callback)
}

func updateJson(ctx context.Context, filepath string, fsys fnfs.ReadWriteFS, callback func(json map[string]interface{}, fileExisted bool)) (map[string]interface{}, error) {
	parsedJson := map[string]interface{}{}

	jsonRaw, err := fs.ReadFile(fsys, filepath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fnerrors.UserError(nil, "error while parsing %s: %s", filepath, err)
	}
	fileExisted := err == nil
	if err == nil {
		if err := json.Unmarshal(jsonRaw, &parsedJson); err != nil {
			return nil, fnerrors.UserError(nil, "error while parsing %s: %s", filepath, err)
		}
	}

	callback(parsedJson, fileExisted)

	updatedJsonRaw, err := json.MarshalIndent(parsedJson, "", "\t")
	if err != nil {
		return nil, err
	}
	// Appending a new line: yarn re-writes package.json every time it reads it and always adds a new line,
	// so for idempotency we do the same.
	updatedJsonRaw = append(updatedJsonRaw, '\n')

	if err := fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), fsys, filepath, func(w io.Writer) error {
		_, err := w.Write(updatedJsonRaw)
		return err
	}); err != nil {
		return nil, err
	}

	return parsedJson, nil
}

func (impl) GenerateServer(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.Definition, error) {
	var dl defs.DefList

	dl.Add("Generate Typescript server dependencies", &OpGenServer{Server: pkg.Server, LoadedNode: nodes}, pkg.PackageName())

	yarnRoot, err := findYarnRoot(pkg.Location)
	if err != nil {
		return nil, err
	}
	dl.Add("Generate Nodejs Yarn root", &OpGenYarnRoot{
		YarnRootPkgName: yarnRoot,
		RelLocation:     pkg.Location.Rel(),
	}, pkg.Location.PackageName)

	return dl.Serialize()
}

func (impl) ParseNode(ctx context.Context, loc workspace.Location, _ *schema.Node, ext *workspace.FrameworkExt) error {
	return nil
}

func (impl) PreParseServer(ctx context.Context, loc workspace.Location, ext *workspace.FrameworkExt) error {
	ext.Include = append(ext.Include, grpcPkg)
	return nil
}

func (impl) PostParseServer(ctx context.Context, sealed *workspace.Sealed) error {
	return nil
}

func (impl) InjectService(loc workspace.Location, node *schema.Node, svc *workspace.CueService) error {
	return nil
}

func (impl) DevelopmentPackages() []schema.PackageName {
	return []schema.PackageName{controllerPkg}
}

func (impl) EvalProvision(*schema.Node) (frontend.ProvisionStack, error) {
	return frontend.ProvisionStack{}, nil
}

func (impl impl) GenerateNode(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.Definition, error) {
	var dl defs.DefList

	maybeGenerateNodeImplStub(pkg, &dl)

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

	yarnRoot, err := findYarnRoot(pkg.Location)
	if err != nil {
		return nil, err
	}
	dl.Add("Generate Nodejs Yarn root", &OpGenYarnRoot{
		YarnRootPkgName: yarnRoot,
		RelLocation:     pkg.Location.Rel(),
	}, pkg.Location.PackageName)

	return dl.Serialize()
}

type yarnRootStatefulGen struct{}

// This is never called but ops.Register requires the Dispatcher.
func (yarnRootStatefulGen) Handle(ctx context.Context, env ops.Environment, _ *schema.Definition, x *OpGenYarnRoot) (*ops.HandleResult, error) {
	return nil, fnerrors.UserError(nil, "yarnRootStatefulGen.Handle is not supposed to be called")
}

func (yarnRootStatefulGen) StartSession(ctx context.Context, env ops.Environment) ops.Session[*OpGenYarnRoot] {
	wenv, ok := env.(workspace.MutableWorkspaceEnvironment)
	if !ok {
		// An error will then be returned in Close().
		wenv = nil
	}

	return &yarnRootGenSession{wenv: wenv, yarnRoots: map[string]context.Context{}}
}

type yarnRootGenSession struct {
	wenv      workspace.MutableWorkspaceEnvironment
	yarnRoots map[string]context.Context
}

func (s *yarnRootGenSession) Handle(ctx context.Context, env ops.Environment, _ *schema.Definition, x *OpGenYarnRoot) (*ops.HandleResult, error) {
	if s.yarnRoots[x.YarnRootPkgName] == nil {
		s.yarnRoots[x.YarnRootPkgName] = ctx
	}
	return nil, nil
}

func (s *yarnRootGenSession) Commit() error {
	// Converting to a slice for deterministic order.
	var roots []string
	for yarnRoot := range s.yarnRoots {
		roots = append(roots, yarnRoot)
	}
	sort.Strings(roots)

	for _, yarnRoot := range roots {
		if err := generateYarnRoot(s.yarnRoots[yarnRoot], yarnRoot, s.wenv.OutputFS()); err != nil {
			return err
		}
	}

	return nil
}

func generateYarnRoot(ctx context.Context, path string, out fnfs.ReadWriteFS) error {
	// Write .yarnrc.yml with the correct nodeLinker.
	if err := fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), out, filepath.Join(path, yarnRcFn), func(w io.Writer) error {
		_, err := io.WriteString(w, yarnRcContent)
		return err
	}); err != nil {
		return err
	}

	// Write .gitignore to hide the foundation plugin.
	if err := fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), out, filepath.Join(path, yarnGitIgnore), func(w io.Writer) error {
		_, err := io.WriteString(w, yarnGitIgnoreContent)
		return err
	}); err != nil {
		return err
	}

	// Write the Foundation plugin.
	if err := fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), out,
		filepath.Join(path, fnYarnPluginPath), func(w io.Writer) error {
			_, err := w.Write(yarnplugin.PluginContent())
			return err
		}); err != nil {
		return err
	}

	// Create "tsconfig.json" if it doesn't exist.
	tsconfigFn := filepath.Join(path, "tsconfig.json")
	_, err := fs.ReadFile(out, tsconfigFn)
	if err != nil && !os.IsNotExist(err) {
		return fnerrors.UserError(nil, "error while parsing %s: %s", tsconfigFn, err)
	}
	fileExisted := err == nil
	if !fileExisted {
		tsConfig := tsConfig{
			Extends: "@tsconfig/node16/tsconfig.json",
		}
		tsConfigRaw, err := json.MarshalIndent(tsConfig, "", "\t")
		if err != nil {
			return err
		}
		if err := fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), out, tsconfigFn, func(w io.Writer) error {
			_, err := w.Write(tsConfigRaw)
			return err
		}); err != nil {
			return err
		}
	}

	return nil
}
