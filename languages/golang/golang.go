// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/gosupport"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/source"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

const (
	grpcNode schema.PackageName = "namespacelabs.dev/foundation/std/go/grpc"
	httpNode schema.PackageName = "namespacelabs.dev/foundation/std/go/http"
)

func Register() {
	languages.Register(schema.Framework_GO, impl{})
	runtime.RegisterSupport(schema.Framework_GO, impl{})

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

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, x *OpGenServer) (*ops.HandleResult, error) {
		wenv, ok := env.(workspace.Packages)
		if !ok {
			return nil, fnerrors.New("workspace.Packages required")
		}

		loc, err := wenv.Resolve(ctx, schema.PackageName(x.Server.PackageName))
		if err != nil {
			return nil, err
		}

		return nil, generateServer(ctx, wenv, loc, x.Server, loc.Module.ReadWriteFS())
	})
}

type impl struct {
	languages.MaybeTidy
	languages.NoDev
}

func (impl) PrepareBuild(ctx context.Context, _ languages.AvailableBuildAssets, server provision.Server, isFocus bool) (build.Spec, error) {
	ext := &FrameworkExt{}
	if err := workspace.MustExtension(server.Proto().Ext, ext); err != nil {
		return nil, fnerrors.Wrap(server.Location, err)
	}

	bin := &GoBinary{
		PackageName:  server.Location.PackageName,
		ModuleName:   server.Module().ModuleName(),
		GoModulePath: ext.GoModulePath,
		GoModule:     ext.GoModule,
		GoVersion:    ext.GoVersion,
		SourcePath:   server.Location.Rel(),
		BinaryName:   "server",
		Capabilities: []string{"grpc", "server"},
		isFocus:      isFocus,
	}

	return bin, nil
}

func (impl) PrepareRun(ctx context.Context, t provision.Server, run *runtime.ServerRunOpts) error {
	run.Command = []string{"/server"}
	run.ReadOnlyFilesystem = true
	run.RunAs = production.NonRootRunAs(production.Distroless)
	return nil
}

func (impl) TidyServer(ctx context.Context, env provision.Env, pkgs workspace.Packages, loc workspace.Location, server *schema.Server) error {
	ext := &FrameworkExt{}
	if err := workspace.MustExtension(server.Ext, ext); err != nil {
		return fnerrors.Wrap(loc, err)
	}

	sdk, err := golang.MatchSDK(ext.GoVersion, golang.HostPlatform())
	if err != nil {
		return fnerrors.Wrap(loc, err)
	}

	localSDK, err := compute.GetValue(ctx, sdk)
	if err != nil {
		return fnerrors.Wrap(loc, err)
	}

	const foundationModule = "namespacelabs.dev/foundation"

	var foundationVersion string
	for _, dep := range loc.Module.Workspace.Dep {
		if dep.ModuleName == foundationModule {
			foundationVersion = dep.Version
		}
	}

	mod, modFile, err := gosupport.LookupGoModule(loc.Abs())
	if err != nil {
		return err
	}

	if foundationVersion != "" {
		// XXX resolve version back to a tag, to support tag based comparison.
		for _, require := range mod.Require {
			if v := require.Mod; v.Path == foundationModule {
				if pr := semver.Prerelease(v.Version); strings.HasPrefix(pr, "-0.") {
					// v0.0.44-0.20220629111102-a3b57dceff40
					// Prelease(): -0.20220629111102-a3b57dceff40
					parts := strings.Split(pr[3:], "-")
					if len(parts) == 2 && len(parts[1]) >= 12 && strings.HasPrefix(foundationVersion, parts[1]) {
						foundationVersion = "" // Nothing to do.
						break
					}
				}
			}
		}

		if foundationVersion != "" {
			if err := RunGo(ctx, loc, localSDK, "get", "-u", fmt.Sprintf("%s@%s", foundationModule, foundationVersion)); err != nil {
				return fnerrors.Wrap(loc, err)
			}
		}
	}

	// Running go mod tidy once per mod file is enough (not once per server).
	abs := filepath.Dir(modFile)
	rel, err := filepath.Rel(loc.Module.Abs(), abs)
	if err != nil {
		return fnerrors.Wrap(loc, err)
	}

	if err := runGo(ctx, rel, abs, localSDK, "mod", "tidy"); err != nil {
		return fnerrors.Wrap(loc, err)
	}

	return nil
}

func (impl) GenerateNode(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.SerializedInvocation, error) {
	var dl defs.DefList

	dl.Add("Generate Go node dependencies", &OpGenNode{
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
		protos, err := protos.Merge(list...)
		if err != nil {
			return nil, err
		}

		dl.Add("Generate Go proto sources", &source.OpProtoGen{
			PackageName: pkg.PackageName().String(),
			Protos:      protos,
			Framework:   schema.Framework_GO,
		})
	}

	return dl.Serialize()
}

func (impl) GenerateServer(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.SerializedInvocation, error) {
	var dl defs.DefList
	dl.Add("Generate Go server dependencies", &OpGenServer{Server: pkg.Server, LoadedNode: nodes}, pkg.PackageName())
	return dl.Serialize()
}

func (impl) PreParseServer(ctx context.Context, loc workspace.Location, ext *workspace.ServerFrameworkExt) error {
	f, gomodFile, err := gosupport.LookupGoModule(loc.Abs())
	if err != nil {
		return err
	}

	if f.Go == nil {
		return fnerrors.BadInputError("%s: no go definition", gomodFile)
	}

	rel, err := filepath.Rel(loc.Module.Abs(), gomodFile)
	if err != nil {
		return err
	}

	ext.FrameworkSpecific, err = anypb.New(&FrameworkExt{
		GoVersion:    f.Go.Version,
		GoModule:     f.Module.Mod.Path,
		GoModulePath: filepath.Dir(rel),
	})
	if err != nil {
		return err
	}

	ext.Include = append(ext.Include, grpcNode)

	return nil
}

func (impl) PostParseServer(ctx context.Context, sealed *workspace.Sealed) error {
	var needGatewayCount int
	for _, dep := range sealed.Deps {
		svc := dep.Service
		if svc == nil {
			continue
		}

		if svc.ExportServicesAsHttp {
			needGatewayCount++
		}

		// XXX this should be done upstream.
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
		return fnerrors.UserError(sealed.Location, "server exposes HTTP paths, it must depend on %s", httpNode)
	}

	return nil
}

func (impl) DevelopmentPackages() []schema.PackageName {
	return nil
}

func packageFrom(loc workspace.Location) (string, error) {
	return gosupport.ComputeGoPackage(loc.Abs())
}

func (impl) InternalEndpoints(_ *schema.Environment, srv *schema.Server, ports []*schema.Endpoint_Port) ([]*schema.InternalEndpoint, error) {
	// XXX have these defined in std/go/core/fn.cue so they're versioned.
	var internals = [][2]string{
		{"/metrics", "prometheus.io/metrics"},
		{"/livez", runtime.FnServiceLivez},
		{"/readyz", runtime.FnServiceReadyz},
	}
	var serverPortName = "server-port"

	var serverPort *schema.Endpoint_Port
	for _, port := range ports {
		if port.Name == serverPortName {
			serverPort = port
			break
		}
	}

	if serverPort == nil {
		// No server port is available during e.g. shutdown.
		return nil, nil
	}

	metadata, err := toServiceMetadata(internals, "http")
	if err != nil {
		return nil, err
	}

	return []*schema.InternalEndpoint{{
		ServerOwner:     srv.PackageName,
		Port:            serverPort,
		ServiceMetadata: metadata,
	}}, nil
}

func toServiceMetadata(internals [][2]string, protocol string) ([]*schema.ServiceMetadata, error) {
	var metadata []*schema.ServiceMetadata
	for _, internal := range internals {
		metrics, err := anypb.New(&schema.HttpExportedService{
			Path: internal[0],
		})
		if err != nil {
			return nil, err
		}

		metadata = append(metadata, &schema.ServiceMetadata{
			Kind:     internal[1],
			Protocol: protocol,
			Details:  metrics,
		})
	}
	return metadata, nil
}
