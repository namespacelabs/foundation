// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/internal/nodejs"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/dev/controller/admin"
	"namespacelabs.dev/foundation/std/web/http"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/source/protos/fnany"
)

const (
	controllerPkg schema.PackageName = "namespacelabs.dev/foundation/std/dev/controller"
	webPkg        schema.PackageName = "namespacelabs.dev/foundation/std/web/http"
	httpPort                         = 10080
	fileSyncPort                     = 50000
	httpPortName                     = "http-port"
	compiledPath                     = "static"
	ForceProd                        = false
)

func Register() {
	languages.Register(schema.Framework_WEB, impl{})
	ops.Register[*OpGenHttpBackend](generator{})
}

type impl struct {
	languages.MaybePrepare
	languages.MaybeGenerate
	languages.MaybeTidy
}

func (impl) PreParseServer(_ context.Context, _ workspace.Location, ext *workspace.ServerFrameworkExt) error {
	return nil
}

func (impl) PostParseServer(ctx context.Context, sealed *workspace.Sealed) error {
	sealed.Proto.Server.StaticPort = []*schema.Endpoint_Port{{Name: httpPortName, ContainerPort: httpPort}}
	return nil
}

func (impl) DevelopmentPackages() []schema.PackageName {
	return []schema.PackageName{controllerPkg}
}

func (impl) PrepareBuild(ctx context.Context, buildAssets languages.AvailableBuildAssets, srv provision.Server, isFocus bool) (build.Spec, error) {
	if useDevBuild(srv.Env().Proto()) {
		pkg, err := srv.Env().LoadByName(ctx, controllerPkg)
		if err != nil {
			return nil, err
		}

		p, err := binary.Plan(ctx, pkg, binary.BuildImageOpts{UsePrebuilts: false})
		if err != nil {
			return nil, err
		}

		return buildDevServer{p.Plan, srv, isFocus, buildAssets.IngressFragments}, nil
	}

	return buildProdWebServer{
		srv, isFocus, buildAssets.IngressFragments,
	}, nil
}

func buildWebApps(ctx context.Context, conf build.BuildTarget, ingressFragments compute.Computable[[]*schema.IngressFragment], srv provision.Server, isFocus bool) ([]compute.Computable[oci.Image], error) {
	var builds []compute.Computable[oci.Image]

	for _, entry := range srv.Proto().UrlMap {
		dep := srv.GetDep(schema.PackageName(entry.PackageName))
		if dep == nil {
			return nil, fnerrors.UserError(srv.Location, "%s: included in url map but not loaded", entry.PackageName)
		}

		backends, err := parseBackends(dep.Node())
		if err != nil {
			return nil, err
		}

		var extra []*memfs.FS
		if len(backends) > 0 {
			fragments, err := compute.GetValue(ctx, ingressFragments)
			if err != nil {
				return nil, fnerrors.InternalError("failed to build web app while waiting on ingress computation: %w", err)
			}

			resolveFunc := resolveBackend(srv.Env(), fragments)
			backend := &OpGenHttpBackend{Backend: backends}
			fsys, err := generateBackendConf(ctx, dep.Location, backend, resolveFunc, false)
			if err != nil {
				return nil, err
			}
			extra = append(extra, fsys)
		}

		loc, err := srv.Env().Resolve(ctx, dep.PackageName())
		if err != nil {
			return nil, err
		}

		targetConf := build.NewBuildTarget(conf.TargetPlatform()).WithTargetName(conf.PublishName()).WithSourcePackage(srv.PackageName())

		b, err := prepareBuild(ctx, loc, srv.Env(), targetConf, entry, isFocus, extra)
		if err != nil {
			return nil, err
		}

		builds = append(builds, b)
	}

	return builds, nil
}

func prepareBuild(ctx context.Context, loc workspace.Location, env ops.Environment, targetConf build.Configuration, entry *schema.Server_URLMapEntry, isFocus bool, extra []*memfs.FS) (compute.Computable[oci.Image], error) {
	if !useDevBuild(env.Proto()) {
		return ViteProductionBuild(ctx, loc, env, targetConf.SourceLabel(), filepath.Join(compiledPath, entry.PathPrefix), entry.PathPrefix, extra...)
	}

	var devwebConfig memfs.FS
	devwebConfig.Add("devweb.config.js", []byte(`import { defineConfig, loadEnv } from "vite";

		export default ({ mode }) => {
		  process.env = { ...process.env, ...loadEnv(mode, process.cwd()) };

		  return defineConfig({
			base: process.env.CMD_DEV_BASE || "/",

			server: {
			  watch: {
				usePolling: true,
				interval: 500,
				binaryInterval: 1000,
			  },
			  hmr: {
				clientPort: process.env.CMD_DEV_PORT,
			  },
			},
		  });
		};`))

	extra = append(extra, &devwebConfig)

	return viteDevBuild(ctx, env, filepath.Join("/packages", entry.PackageName), loc, isFocus, targetConf, extra...)
}

func (impl) PrepareDev(ctx context.Context, srv provision.Server) (context.Context, languages.DevObserver, error) {
	if wsremote.Ctx(ctx) != nil {
		return nil, nil, fnerrors.UserError(srv.Location, "`ns dev` on multiple web/nodejs servers not supported")
	}

	devObserver := hotreload.NewFileSyncDevObserver(ctx, srv, fileSyncPort)

	newCtx, _ := wsremote.WithRegistrar(ctx, devObserver.Deposit)

	return newCtx, devObserver, nil
}

func (impl) PrepareRun(ctx context.Context, srv provision.Server, run *runtime.ServerRunOpts) error {
	if useDevBuild(srv.Env().Proto()) {
		configuration := &admin.Configuration{
			PackageBase:  "/packages",
			RevproxyPort: httpPort,
			FilesyncPort: fileSyncPort,
		}

		for k, m := range srv.Proto().UrlMap {
			port := httpPort + k + 1
			configuration.Backend = append(configuration.Backend, &admin.Backend{
				PackageName: m.PackageName,
				Execution: &admin.Execution{
					Args: []string{
						"node_modules/vite/bin/vite.js",
						"--config=devweb.config.js",
						"--clearScreen=false",
						"--host=127.0.0.1",
						fmt.Sprintf("--port=%d", port),
					},
					AdditionalEnv: []string{
						fmt.Sprintf("CMD_DEV_BASE=%s", m.PathPrefix),
						fmt.Sprintf("CMD_DEV_PORT=%d", runtime.LocalIngressPort),
					},
				},
				HttpPass: &admin.HttpPass{
					UrlPrefix: m.PathPrefix,
					Backend:   fmt.Sprintf("127.0.0.1:%d", port),
				},
			})
		}

		serialized, err := protojson.Marshal(configuration)
		if err != nil {
			return fnerrors.InternalError("failed to serialize configuration: %v", err)
		}

		// XXX support a persistent cache: https://vitejs.dev/guide/dep-pre-bundling.html#file-system-cache

		run.Command = []string{"/devcontroller"}
		run.Args = append(run.Args, fmt.Sprintf("--configuration=%s", serialized))
		return nil
	}

	run.Command = []string{"nginx", "-g", "daemon off;"}

	// This is OK because the nginx image output logs to stdout/stderr by default.
	run.ReadOnlyFilesystem = false // See #276.

	// See #276.
	// run.RunAs = &runtime.RunAs{
	// 	UserID: "101", // This is the image's default. We lift it here explicitly for visibility at the runtime level.
	// }
	return nil
}

func useDevBuild(env *schema.Environment) bool {
	return !ForceProd && env.Purpose == schema.Environment_DEVELOPMENT
}

func (i impl) TidyNode(ctx context.Context, env provision.Env, pkgs workspace.Packages, p *workspace.Package) error {
	if p.Node().Kind != schema.Node_SERVICE {
		return nil
	}

	devPackages := []string{
		"typescript@4.5.4",
		"vite@2.7.13",
	}

	if err := nodejs.RunYarn(ctx, env, p.Location.Rel(), append([]string{"add", "-D", "--mode=skip-build"}, devPackages...), p.Location.Module.WorkspaceData); err != nil {
		return err
	}

	return nil
}

func parseBackends(n *schema.Node) ([]*OpGenHttpBackend_Backend, error) {
	var backends []*OpGenHttpBackend_Backend
	for _, p := range n.Instantiate {
		backend := &http.Backend{}

		if ok, err := fnany.CheckUnmarshal(p.Constructor, webPkg, backend); ok {
			if err != nil {
				return nil, err
			}

			backends = append(backends, &OpGenHttpBackend_Backend{
				InstanceName:  p.Name,
				EndpointOwner: backend.EndpointOwner,
				ServiceName:   backend.ServiceName,
				IngressName:   backend.IngressName,
				Manager:       backend.Manager,
			})
		}
	}

	return backends, nil
}

func (i impl) GenerateNode(pkg *workspace.Package, available []*schema.Node) ([]*schema.SerializedInvocation, error) {
	var dl defs.DefList

	backends, err := parseBackends(pkg.Node())
	if err != nil {
		return nil, err
	}

	if len(backends) > 0 {
		dl.Add("Generate Web node dependencies", &OpGenHttpBackend{
			Node:    pkg.Node(),
			Backend: backends,
		}, pkg.PackageName())
	}

	return dl.Serialize()
}

type buildDevServer struct {
	baseImage        build.Plan
	srv              provision.Server
	isFocus          bool
	ingressFragments compute.Computable[[]*schema.IngressFragment]
}

func (bws buildDevServer) BuildImage(ctx context.Context, env ops.Environment, conf build.Configuration) (compute.Computable[oci.Image], error) {
	builds, err := buildWebApps(ctx, conf, bws.ingressFragments, bws.srv, bws.isFocus)
	if err != nil {
		return nil, err
	}

	baseImage, err := bws.baseImage.Spec.BuildImage(ctx, env,
		build.NewBuildTarget(conf.TargetPlatform()).
			WithTargetName(conf.PublishName()).
			WithSourceLabel(bws.baseImage.SourceLabel).
			WithWorkspace(bws.baseImage.Workspace))
	if err != nil {
		return nil, err
	}

	images := []compute.Computable[oci.Image]{baseImage}
	images = append(images, builds...)

	return oci.MergeImageLayers(images...), nil
}

func (bws buildDevServer) PlatformIndependent() bool { return false }

type buildProdWebServer struct {
	srv              provision.Server
	isFocus          bool
	ingressFragments compute.Computable[[]*schema.IngressFragment]
}

func (bws buildProdWebServer) BuildImage(ctx context.Context, env ops.Environment, conf build.Configuration) (compute.Computable[oci.Image], error) {
	builds, err := buildWebApps(ctx, conf, bws.ingressFragments, bws.srv, bws.isFocus)
	if err != nil {
		return nil, err
	}

	// XXX this is not quite right. We always setup a index.html fallback
	// regardless of content. And that's probably over-reaching. The user should
	// let us know which paths require this fallback.
	var defaultConf memfs.FS
	defaultConf.Add("etc/nginx/conf.d/default.conf", []byte(fmt.Sprintf(`server {
		listen %d;
		server_name localhost;

		location / {
			root /%s;
			index index.html;
			try_files $uri /index.html;
		}

		error_page 500 502 503 504 /50x.html;
		location = /50x.html {
			root /usr/share/nginx/html;
		}
}`, httpPort, compiledPath)))
	config := oci.MakeLayer("conf", compute.Precomputed[fs.FS](&defaultConf, digestfs.Digest))

	images := []compute.Computable[oci.Image]{
		oci.ResolveImage("nginx:1.21.5-alpine", *conf.TargetPlatform()),
		oci.MakeImage(oci.Scratch(), config),
	}
	images = append(images, builds...)
	return oci.MergeImageLayers(images...), nil
}

// This is unfortunate, but because of our layering we do indeed end up building images twice.
func (bws buildProdWebServer) PlatformIndependent() bool { return false }
