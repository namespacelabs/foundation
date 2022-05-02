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
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protojson"
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
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/dev/controller/admin"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

// Hard-coding the version of generated yarn workspaces since we only support a monorepo for now.
const (
	controllerPkg        schema.PackageName = "namespacelabs.dev/foundation/std/dev/controller"
	grpcPkg              schema.PackageName = "namespacelabs.dev/foundation/std/web/http"
	yarnWorkspaceVersion                    = "0.0.0"
	yarnVersion                             = "3.2.0"
	yarnRcFn                                = ".yarnrc.yml"
	implFileName                            = "impl.ts"
	packageJsonFn                           = "package.json"
	fileSyncPort                            = 50000
	serverPort                              = 10090
	runtimePackage                          = "@namespacelabs/foundation"
	// This has the specific value to make the ingress code to do port forwarding and expose this port.
	// TODO(@nicolasalt): expose individual gRPC services instead when extensions are supported.
	serverPortName = "server-port"
	ForceProd      = false
)

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

func useDevBuild(env *schema.Environment) bool {
	// TODO(@nicolasalt): uncomment when #313 is fixed.
	// Currently only dev way to pass flags to the server works, see PostParseServer.
	// return !ForceProd && env.Purpose == schema.Environment_DEVELOPMENT
	return true
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

	if err := generateServer(ctx, workspacePackages, loc, msg.Server, msg.LoadedNode, loc.Module.ReadWriteFS()); err != nil {
		return nil, fnerrors.InternalError("failed to generate server: %w", err)
	}

	return nil, nil
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
			Sink: r.For(&wsremote.Signature{ModuleName: "", Rel: yarnRoot.String()}),
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
	if wsremote.Ctx(ctx) != nil {
		return nil, nil, fnerrors.UserError(srv.Location, "`fn dev` on multiple web/nodejs servers not supported")
	}

	devObserver := hotreload.NewFileSyncDevObserver(ctx, srv, fileSyncPort)

	newCtx, _ := wsremote.WithRegistrar(ctx, devObserver.Deposit)

	return newCtx, devObserver, nil
}

func (impl) PrepareRun(ctx context.Context, srv provision.Server, run *runtime.ServerRunOpts) error {
	if useDevBuild(srv.Env().Proto()) {
		// For dev builds we use runtime complication of Typescript.
		run.ReadOnlyFilesystem = false

		// Initialize devcontroller

		configuration := &admin.Configuration{
			PackageBase:  "/app",
			FilesyncPort: fileSyncPort,
			Backend: []*admin.Backend{{
				// To match the file sync event package name.
				PackageName: ".",
				Execution: &admin.Execution{
					Args: []string{
						"nodemon",
						filepath.Join(srv.Location.Rel(), "main.fn.ts"),
						"--listen_hostname=127.0.0.1",
						fmt.Sprintf("--port=%d", serverPort),
					},
				},
			}},
		}
		serialized, err := protojson.Marshal(configuration)
		if err != nil {
			return fnerrors.InternalError("failed to serialize configuration: %v", err)
		}
		run.Command = []string{"/devcontroller"}
		run.Args = append(run.Args, fmt.Sprintf("--configuration=%s", serialized))
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

func (impl) TidyWorkspace(ctx context.Context, packages []*workspace.Package) error {
	yarnRoots := map[string]*workspace.Module{}
	for _, pkg := range packages {
		if (pkg.Server != nil && pkg.Server.Framework == schema.Framework_NODEJS) ||
			(pkg.Node() != nil && slices.Contains(pkg.Node().CodegeneratedFrameworks(), schema.Framework_NODEJS)) {
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
		if err := tidyYarnRoot(ctx, yarnRoot, module); err != nil {
			return err
		}
	}

	return nil
}

func tidyYarnRoot(ctx context.Context, path string, module *workspace.Module) error {
	yarnHasCorrectVersion, err := updateYarnRootPackageJson(ctx, path, module.ReadWriteFS())

	if err != nil {
		return err
	}

	// Install Yarn 3+ if needed
	if !yarnHasCorrectVersion {
		if err := yarn.RunYarn(ctx, path, []string{"set", "version", yarnVersion}); err != nil {
			return err
		}
		// Yarn adds "packageManager" field to the end of package.json, re-format it in alphapetical order
		// to make tn tidy idempotent.
		if _, err = updateYarnRootPackageJson(ctx, path, module.ReadWriteFS()); err != nil {
			return err
		}
	}

	// Write .yarnrc.yml with the correct nodeLinker.
	if err := fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), module.ReadWriteFS(), filepath.Join(path, yarnRcFn), func(w io.Writer) error {
		_, err := io.WriteString(w, yarnRcContent())
		return err
	}); err != nil {
		return err
	}

	// Create "tsconfig.json" if it doesn't exist.
	tsconfigFn := filepath.Join(path, "tsconfig.json")
	if _, err := updateJson(ctx, tsconfigFn, module.ReadWriteFS(),
		func(tsconfig map[string]interface{}, fileExisted bool) {
			if !fileExisted {
				tsconfig["extends"] = "@tsconfig/node16/tsconfig.json"
				tsconfig["compilerOptions"] = map[string]interface{}{
					"sourceMap": true,
				}
			}
		}); err != nil {
		return err
	}

	// `fn tidy` could update dependencies of some nodes/servers, running `yarn install` to update
	// `node_modules`.
	if err := yarn.RunYarn(ctx, path, []string{"install"}); err != nil {
		return err
	}

	return nil
}

func yarnRcContent() string {
	return fmt.Sprintf(
		`nodeLinker: node-modules

npmScopes: 
  namespacelabs:
    npmRegistryServer: "https://us-npm.pkg.dev/foundation-344819/npm-prebuilts/"

yarnPath: .yarn/releases/yarn-%s.cjs
`, yarnVersion)
}

// Returns whether yarn has the correct version
func updateYarnRootPackageJson(ctx context.Context, path string, fs fnfs.ReadWriteFS) (bool, error) {
	yarnHasCorrectVersion := false
	_, err := updatePackageJson(ctx, path, fs, func(packageJson map[string]interface{}, fileExisted bool) {
		packageJson["private"] = true
		packageJson["workspaces"] = []string{"**/*"}
		yarnWithVersion := fmt.Sprintf("yarn@%s", yarnVersion)
		yarnHasCorrectVersion = packageJson["packageManager"] == yarnWithVersion
	})

	return yarnHasCorrectVersion, err
}

func (impl) TidyNode(ctx context.Context, pkgs workspace.Packages, p *workspace.Package) error {
	depPkgNames := []string{}
	for _, ref := range p.Node().Reference {
		if ref.PackageName != "" {
			depPkgNames = append(depPkgNames, ref.PackageName)
		}
	}

	err := tidyPackageJson(ctx, pkgs, p.Location, depPkgNames)
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

	tmplOptions := nodeImplTmplOptions{}
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

	return generateSource(ctx, p.Location.Module.ReadWriteFS(), implFn, tmpl, "Node stub", tmplOptions)
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
	npmPackage, err := toNpmPackage(loc.PackageName)
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
			importNpmPackage, err := toNpmPackage(pkg.PackageName())
			if err != nil {
				return err
			}
			dependencies[string(importNpmPackage)] = yarnWorkspaceVersion
		}
	}

	_, err = updatePackageJson(ctx, loc.Rel(), loc.Module.ReadWriteFS(), func(packageJson map[string]interface{}, fileExisted bool) {
		packageJson["name"] = npmPackage
		packageJson["private"] = true
		packageJson["version"] = yarnWorkspaceVersion

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
		return nil, err
	}
	fileExisted := err == nil
	if err == nil {
		if err := json.Unmarshal(jsonRaw, &parsedJson); err != nil {
			return nil, err
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
	return dl.Serialize()
}

func (impl) ParseNode(ctx context.Context, loc workspace.Location, _ *schema.Node, ext *workspace.FrameworkExt) error {
	return nil
}

func (impl) PreParseServer(ctx context.Context, loc workspace.Location, ext *workspace.FrameworkExt) error {
	ext.Include = append(ext.Include, grpcPkg, controllerPkg)
	return nil
}

func (impl) PostParseServer(ctx context.Context, sealed *workspace.Sealed) error {
	sealed.Proto.Server.StaticPort = []*schema.Endpoint_Port{{Name: serverPortName, ContainerPort: serverPort}}
	return nil
}

func (impl) InjectService(loc workspace.Location, node *schema.Node, svc *workspace.CueService) error {
	return nil
}

func (impl) DevelopmentPackages() []schema.PackageName {
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
