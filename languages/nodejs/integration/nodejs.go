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
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/internal/nodejs"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

const (
	controllerPkg schema.PackageName = "namespacelabs.dev/foundation/std/development/filesync/controller"
	grpcNode      schema.PackageName = "namespacelabs.dev/foundation/std/nodejs/grpc"
	httpNode      schema.PackageName = "namespacelabs.dev/foundation/std/nodejs/http"
	runtimeNode   schema.PackageName = "namespacelabs.dev/foundation/std/nodejs/runtime"
	implFileName                     = "impl.ts"
	packageJsonFn                    = "package.json"
	fileSyncPort                     = 50000
	ForceProd                        = false
)

var ()

func Register() {
	languages.Register(schema.Framework_NODEJS, impl{})

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, x *OpGenServer) (*ops.HandleResult, error) {
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

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, x *OpGenNode) (*ops.HandleResult, error) {
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

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, x *OpGenNodeStub) (*ops.HandleResult, error) {
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

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, x *OpGenGrpc) (*ops.HandleResult, error) {
		wenv, ok := env.(workspace.Packages)
		if !ok {
			return nil, fnerrors.New("workspace.Packages required")
		}

		loc, err := wenv.Resolve(ctx, schema.PackageName(x.PackageName))
		if err != nil {
			return nil, err
		}

		return nil, generateGrpcApi(ctx, x.Protos, loc)
	})

	ops.Register[*OpGenYarnRoot](yarnRootStatefulGen{})
}

func useDevBuild(env *schema.Environment) bool {
	return !ForceProd && env.Purpose == schema.Environment_DEVELOPMENT
}

type impl struct {
	languages.MaybeGenerate
	languages.MaybeTidy
	languages.NoDev
}

func GetExternalModuleForDeps(server provision.Server) []build.Workspace {
	moduleMap := map[string]*workspace.Module{}
	for _, dep := range server.Deps() {
		if dep.Location.Module.ModuleName() != server.Module().ModuleName() &&
			(dep.Node() != nil && (slices.Contains(dep.Node().CodegeneratedFrameworks(), schema.Framework_NODEJS) ||
				slices.Contains(dep.Node().CodegeneratedFrameworks(), schema.Framework_WEB))) {
			moduleMap[dep.Location.Module.ModuleName()] = dep.Location.Module
		}
	}
	modules := []build.Workspace{}
	for _, module := range moduleMap {
		modules = append(modules, module)
	}

	return modules
}

func (impl) PrepareBuild(ctx context.Context, _ languages.AvailableBuildAssets, server provision.Server, isFocus bool) (build.Spec, error) {
	yarnRoot, err := findYarnRoot(server.Location)
	if err != nil {
		return nil, err
	}

	isDevBuild := useDevBuild(server.Env().Proto())

	var module build.Workspace
	if r := wsremote.Ctx(ctx); r != nil && isFocus && !server.Location.Module.IsExternal() && isDevBuild {
		module = nodejs.YarnHotReloadModule{
			Module: server.Location.Module,
			// "ModuleName" is empty because we have only one module in the image and
			// we can put everything under the root "/app" directory.
			Sink: r.For(&wsremote.Signature{ModuleName: "", Rel: yarnRoot}),
		}
	} else {
		module = server.Location.Module
	}

	return buildNodeJS{
		module:          module,
		workspace:       server.Location.Module.Workspace,
		externalModules: GetExternalModuleForDeps(server),
		yarnRoot:        yarnRoot,
		serverEnv:       server.Env(),
		isDevBuild:      isDevBuild,
		isFocus:         isFocus,
	}, nil
}

func pkgSupportsNodejs(pkg *workspace.Package) bool {
	return (pkg.Server != nil && pkg.Server.Framework == schema.Framework_NODEJS) ||
		(pkg.Node() != nil && slices.Contains(pkg.Node().CodegeneratedFrameworks(), schema.Framework_NODEJS))
}

func (impl) PrepareDev(ctx context.Context, srv provision.Server) (context.Context, languages.DevObserver, error) {
	if useDevBuild(srv.Env().Proto()) {
		if wsremote.Ctx(ctx) != nil {
			return nil, nil, fnerrors.UserError(srv.Location, "`ns dev` on multiple web/nodejs servers not supported")
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
	module         *workspace.Module
	workspacePaths []string
	workspace      *schema.Workspace
}

func (impl) TidyWorkspace(ctx context.Context, env provision.Env, packages []*workspace.Package) error {
	yarnRootsMap := map[string]*yarnRootData{}
	yarnRoots := []string{}
	for _, pkg := range packages {
		if pkgSupportsNodejs(pkg) {
			yarnRoot, err := findYarnRoot(pkg.Location)
			if err != nil {
				// If we can't find yarn root, using the workspace root.
				yarnRoot = ""
			}
			if yarnRootsMap[yarnRoot] == nil {
				yarnRootsMap[yarnRoot] = &yarnRootData{
					module:         pkg.Location.Module,
					workspacePaths: []string{},
					workspace:      pkg.Location.Module.Workspace,
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
		// Can't fail
		yarnRootData := yarnRootsMap[yarnRoot]

		if err := updateYarnRootPackageJson(ctx, yarnRootData, yarnRoot); err != nil {
			return err
		}

		// `ns tidy` could update dependencies of some nodes/servers, running `yarn install` to update
		// `node_modules`.
		if err := nodejs.RunYarn(ctx, env, yarnRoot, []string{"install", "--mode=skip-build"}, yarnRootData.module.WorkspaceData); err != nil {
			return err
		}
	}

	return nil
}

func updateYarnRootPackageJson(ctx context.Context, yarnRootData *yarnRootData, path string) error {
	dependencies := map[string]string{}
	for k, v := range builtin().Dependencies {
		dependencies[k] = v
	}
	for _, moduleName := range yarnRootData.workspace.AllReferencedModules() {
		dependencies[toNpmNamespace(moduleName)] = "fn:" + moduleName
	}

	_, err := updatePackageJson(ctx, path, yarnRootData.module.ReadWriteFS(), func(packageJson map[string]interface{}, fileExisted bool) {
		packageJson["private"] = true
		packageJson["name"] = toNpmNamespace(yarnRootData.workspace.ModuleName)

		packageJson["dependencies"] = mergeJsonMap(packageJson["dependencies"], dependencies)
		packageJson["devDependencies"] = mergeJsonMap(packageJson["devDependencies"], builtin().DevDependencies)
	})

	return err
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
		tmplOptions.ServiceServerName = fmt.Sprintf("%sServer", srvName)
		tmplOptions.DefineServiceFunName = fmt.Sprintf("define%sServer", srvName)

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

func (impl) GenerateServer(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.SerializedInvocation, error) {
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

func (impl) PreParseServer(ctx context.Context, loc workspace.Location, ext *workspace.ServerFrameworkExt) error {
	// Adding extra nodes here:
	// - grpcNode sets up correct flags for the server startup.
	// - runtimeNode allows to treat the Namespace Node.js runtime as a regular node that has a Location,
	// and copy it to the build image in the same way as other nodes.
	ext.Include = append(ext.Include, grpcNode, httpNode, runtimeNode)
	return nil
}

// TODO: consolidate with the Go implementation.
func (impl) PostParseServer(ctx context.Context, sealed *workspace.Sealed) error {
	for _, dep := range sealed.Deps {
		svc := dep.Service
		if svc == nil {
			continue
		}

		for _, p := range svc.ExportHttp {
			sealed.Proto.Server.UrlMap = append(sealed.Proto.Server.UrlMap, &schema.Server_URLMapEntry{
				PathPrefix:  p.Path,
				IngressName: svc.IngressServiceName,
				Kind:        p.Kind,
				PackageName: svc.PackageName,
			})
		}
	}

	if len(sealed.Proto.Server.UrlMap) > 0 && !sealed.HasDep(httpNode) {
		return fnerrors.UserError(sealed.Location, "node.js server exposes HTTP paths, it must depend on %s", httpNode)
	}

	return nil
}

func (impl) DevelopmentPackages() []schema.PackageName {
	return []schema.PackageName{controllerPkg}
}

func (impl impl) GenerateNode(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.SerializedInvocation, error) {
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
		merged, err := protos.Merge(list...)
		if err != nil {
			return nil, err
		}
		dl.Add("Generate Typescript proto sources", &source.OpProtoGen{
			PackageName: pkg.PackageName().String(),
			Protos:      merged,
			Framework:   schema.Framework_NODEJS,
		})

		dl.Add("Generate Typescript gRPC proto sources", &OpGenGrpc{
			PackageName: pkg.PackageName().String(),
			Protos:      merged,
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
func (yarnRootStatefulGen) Handle(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, x *OpGenYarnRoot) (*ops.HandleResult, error) {
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

func (s *yarnRootGenSession) Handle(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, x *OpGenYarnRoot) (*ops.HandleResult, error) {
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
