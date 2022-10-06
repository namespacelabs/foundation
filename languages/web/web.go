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
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/internal/nodejs"
	"namespacelabs.dev/foundation/languages"
	nodejsintegration "namespacelabs.dev/foundation/languages/nodejs/integration"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/provision/parsed"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/development/controller/admin"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/web/http"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos/fnany"
)

var (
	controllerPkg = schema.MakePackageSingleRef("namespacelabs.dev/foundation/std/development/controller")
)

const (
	httpPort                        = 40000
	fileSyncPort                    = 50000
	httpPortName                    = "http-port"
	compiledPath                    = "static"
	webPkg       schema.PackageName = "namespacelabs.dev/foundation/std/web/http"
)

func Register() {
	languages.Register(schema.Framework_WEB, impl{})
	ops.RegisterHandlerFunc(generateWebBackend)
}

type impl struct {
	languages.MaybePrepare
	languages.MaybeGenerate
	languages.MaybeTidy
}

func (impl) PreParseServer(_ context.Context, _ pkggraph.Location, ext *workspace.ServerFrameworkExt) error {
	return nil
}

func (impl) PostParseServer(ctx context.Context, sealed *workspace.Sealed) error {
	sealed.Proto.Server.StaticPort = []*schema.Endpoint_Port{{Name: httpPortName, ContainerPort: httpPort}}
	return nil
}

func (impl) DevelopmentPackages() []schema.PackageName {
	return []schema.PackageName{controllerPkg.AsPackageName()}
}

func (impl) PrepareBuild(ctx context.Context, buildAssets languages.AvailableBuildAssets, srv parsed.Server, isFocus bool) (build.Spec, error) {
	if opaque.UseDevBuild(srv.SealedContext().Environment()) {
		pkg, err := srv.SealedContext().LoadByName(ctx, controllerPkg.AsPackageName())
		if err != nil {
			return nil, err
		}

		p, err := binary.Plan(ctx, pkg, controllerPkg.Name, srv.SealedContext(), binary.BuildImageOpts{UsePrebuilts: false})
		if err != nil {
			return nil, err
		}

		return buildDevServer{p.Plan, srv, isFocus, buildAssets.IngressFragments}, nil
	}

	return buildProdWebServer{
		srv, isFocus, buildAssets.IngressFragments,
	}, nil
}

func buildWebApps(ctx context.Context, conf build.BuildTarget, ingressFragments compute.Computable[[]*schema.IngressFragment], srv parsed.Server, isFocus bool) ([]oci.NamedImage, error) {
	var builds []oci.NamedImage

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

			resolveFunc := resolveBackend(srv.SealedContext(), fragments)
			backend := &OpGenHttpBackend{Backend: backends}
			fsys, err := generateBackendConf(ctx, dep.Location, backend, resolveFunc, false)
			if err != nil {
				return nil, err
			}
			extra = append(extra, fsys)
		}

		loc, err := srv.SealedContext().Resolve(ctx, dep.PackageName())
		if err != nil {
			return nil, err
		}

		targetConf := build.NewBuildTarget(conf.TargetPlatform()).WithTargetName(conf.PublishName()).WithSourcePackage(srv.PackageName())

		externalModules := nodejsintegration.GetExternalModuleForDeps(srv)

		b, err := prepareBuild(ctx, loc, srv.SealedContext(), targetConf, entry, isFocus, externalModules, extra)
		if err != nil {
			return nil, err
		}

		builds = append(builds, b)
	}

	return builds, nil
}

func prepareBuild(ctx context.Context, loc pkggraph.Location, env planning.Context, targetConf build.Configuration, entry *schema.Server_URLMapEntry, isFocus bool, externalModules []build.Workspace, extra []*memfs.FS) (oci.NamedImage, error) {
	if !opaque.UseDevBuild(env.Environment()) {

		extra = append(extra, generateProdViteConfig())

		return ViteProductionBuild(ctx, loc, env, targetConf.SourceLabel(), filepath.Join(compiledPath, entry.PathPrefix), entry.PathPrefix, externalModules, extra...)
	}

	var devwebConfig memfs.FS
	devwebConfig.Add("devweb.config.js", []byte(`
	  import { defineConfig, loadEnv } from "vite";
	  import react from "@vitejs/plugin-react";
		import pluginRewriteAll from "vite-plugin-rewrite-all";

		export default ({ mode }) => {
		  process.env = { ...process.env, ...loadEnv(mode, process.cwd()) };

		  return defineConfig({
    		plugins: [react(), pluginRewriteAll()],

				base: process.env.CMD_DEV_BASE || "/",

				resolve: {
					// Important to correctly handle ns-managed dependencies.
					preserveSymlinks: true,
				},

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

	return viteDevBuild(ctx, env, filepath.Join("/packages", loc.Module.ModuleName()), loc, isFocus, targetConf, externalModules, extra...)
}

func generateProdViteConfig() *memfs.FS {
	var prodwebConfig memfs.FS
	prodwebConfig.Add("prodweb.config.js", []byte(`
	    import react from "@vitejs/plugin-react";

			export default {
    		plugins: [react()],

				resolve: {
					// Important to correctly handle ns-managed dependencies.
					preserveSymlinks: true,
				},
			}`))
	return &prodwebConfig
}

func (impl) PrepareDev(ctx context.Context, cluster runtime.ClusterNamespace, srv parsed.Server) (context.Context, languages.DevObserver, error) {
	return hotreload.ConfigureFileSyncDevObserver(ctx, cluster, srv)
}

func (impl) PrepareRun(ctx context.Context, srv parsed.Server, run *runtime.ContainerRunOpts) error {
	if opaque.UseDevBuild(srv.SealedContext().Environment()) {
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

func (i impl) TidyNode(ctx context.Context, env planning.Context, pkgs pkggraph.PackageLoader, p *pkggraph.Package) error {
	devPackages := []string{
		"typescript@4.5.4",
	}

	if p.Node().Kind == schema.Node_SERVICE {
		devPackages = append(devPackages, "vite@2.7.13")
	}

	if p.Location.Module.IsExternal() {
		return fnerrors.BadInputError("%s: can't run tidy on external module", p.Location.Module)
	}

	if err := nodejs.RunYarn(ctx, env, p.Location, append([]string{"add", "-D", "--mode=skip-build"}, devPackages...)); err != nil {
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

func (i impl) GenerateNode(pkg *pkggraph.Package, available []*schema.Node) ([]*schema.SerializedInvocation, error) {
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
	srv              parsed.Server
	isFocus          bool
	ingressFragments compute.Computable[[]*schema.IngressFragment]
}

func (bws buildDevServer) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
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

	images := []oci.NamedImage{oci.MakeNamedImage(bws.baseImage.SourceLabel, baseImage)}
	images = append(images, builds...)

	return oci.MergeImageLayers(images...), nil
}

func (bws buildDevServer) PlatformIndependent() bool { return false }

type buildProdWebServer struct {
	srv              parsed.Server
	isFocus          bool
	ingressFragments compute.Computable[[]*schema.IngressFragment]
}

func (bws buildProdWebServer) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
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

	images := []oci.NamedImage{
		oci.ResolveImage("nginx:1.21.5-alpine", *conf.TargetPlatform()),
		oci.MakeImageFromScratch("nginx-configuration", config),
	}
	images = append(images, builds...)
	return oci.MergeImageLayers(images...), nil
}

// This is unfortunate, but because of our layering we do indeed end up building images twice.
func (bws buildProdWebServer) PlatformIndependent() bool { return false }
