// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/assets"
	source "namespacelabs.dev/foundation/internal/codegen"
	"namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/gosupport"
	"namespacelabs.dev/foundation/internal/integrations"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/execution/defs"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	grpcNode schema.PackageName = "namespacelabs.dev/foundation/std/go/grpc"
	httpNode schema.PackageName = "namespacelabs.dev/foundation/std/go/http"
)

func Register() {
	integrations.Register(schema.Framework_GO, impl{})
	planning.RegisterEndpointProvider(schema.Framework_GO, impl{})

	execution.RegisterFuncs(execution.Funcs[*OpGenNode]{
		Handle: func(ctx context.Context, _ *schema.SerializedInvocation, x *OpGenNode) (*execution.HandleResult, error) {
			loader, err := execution.Get(ctx, pkggraph.PackageLoaderInjection)
			if err != nil {
				return nil, err
			}

			loc, err := loader.Resolve(ctx, schema.PackageName(x.Node.PackageName))
			if err != nil {
				return nil, err
			}

			return nil, generateNode(ctx, loader, loc, x.Node, x.LoadedNode, loc.Module.ReadWriteFS())
		},
	})

	execution.RegisterFuncs(execution.Funcs[*OpGenServer]{
		Handle: func(ctx context.Context, _ *schema.SerializedInvocation, x *OpGenServer) (*execution.HandleResult, error) {
			loader, err := execution.Get(ctx, pkggraph.PackageLoaderInjection)
			if err != nil {
				return nil, err
			}

			loc, err := loader.Resolve(ctx, schema.PackageName(x.Server.PackageName))
			if err != nil {
				return nil, err
			}

			return nil, generateServer(ctx, loader, loc, x.Server, loc.Module.ReadWriteFS())
		},
	})
}

type impl struct {
	integrations.MaybeTidy
	integrations.NoDev
}

func (impl) PrepareBuild(ctx context.Context, _ assets.AvailableBuildAssets, server planning.Server, isFocus bool) (build.Spec, error) {
	ext := &FrameworkExt{}
	if err := parsing.MustExtension(server.Proto().Ext, ext); err != nil {
		return nil, fnerrors.AttachLocation(server.Location, err)
	}

	bin := &GoBinary{
		PackageName:  server.Location.PackageName,
		GoModulePath: ext.GoModulePath,
		GoModule:     ext.GoModule,
		GoVersion:    ext.GoVersion,
		SourcePath:   server.Location.Rel(),
		BinaryName:   "server",
		isFocus:      isFocus,
	}

	return bin, nil
}

func (impl) PrepareRun(ctx context.Context, t planning.Server, run *runtime.ContainerRunOpts) error {
	run.Command = []string{"/server"}
	run.ReadOnlyFilesystem = true
	run.RunAs = production.NonRootRunAs(production.Distroless)
	return nil
}

func (impl) TidyServer(ctx context.Context, env cfg.Context, pkgs pkggraph.PackageLoader, loc pkggraph.Location, server *schema.Server) error {
	ext := &FrameworkExt{}
	if err := parsing.MustExtension(server.Ext, ext); err != nil {
		return fnerrors.AttachLocation(loc, err)
	}

	sdk, err := golang.MatchSDK(ext.GoVersion, host.HostPlatform())
	if err != nil {
		return fnerrors.AttachLocation(loc, err)
	}

	localSDK, err := compute.GetValue(ctx, sdk)
	if err != nil {
		return fnerrors.AttachLocation(loc, err)
	}

	const foundationModule = "namespacelabs.dev/foundation"

	var foundationVersion string
	for _, dep := range loc.Module.Workspace.Dep {
		if dep.ModuleName == foundationModule {
			foundationVersion = dep.Version
		}
	}

	if foundationVersion != "" {
		mod, _, err := gosupport.LookupGoModule(loc.Abs())
		if err != nil {
			return err
		}

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
				return fnerrors.AttachLocation(loc, err)
			}
		}
	}

	if err := RunGo(ctx, loc, localSDK, "mod", "tidy"); err != nil {
		return fnerrors.AttachLocation(loc, err)
	}

	return nil
}

func (impl) GenerateNode(pkg *pkggraph.Package, nodes []*schema.Node) ([]*schema.SerializedInvocation, error) {
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

func (impl) GenerateServer(pkg *pkggraph.Package, nodes []*schema.Node) ([]*schema.SerializedInvocation, error) {
	var dl defs.DefList
	dl.Add("Generate Go server dependencies", &OpGenServer{Server: pkg.Server, LoadedNode: nodes}, pkg.PackageName())
	return dl.Serialize()
}

func (impl) PreParseServer(ctx context.Context, loc pkggraph.Location, ext *parsing.ServerFrameworkExt) error {
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

func (impl) PostParseServer(ctx context.Context, sealed *parsing.Sealed) error {
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
		return fnerrors.NewWithLocation(sealed.Location, "server exposes HTTP paths, it must depend on %s", httpNode)
	}

	return nil
}

func (impl) DevelopmentPackages() []schema.PackageName {
	return nil
}

func packageFrom(loc pkggraph.Location) (string, error) {
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
