// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/gosupport"
	"namespacelabs.dev/foundation/internal/localexec"
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
	"namespacelabs.dev/foundation/workspace/tasks"
)

const (
	grpcNode    schema.PackageName = "namespacelabs.dev/foundation/std/go/grpc"
	gatewayNode schema.PackageName = "namespacelabs.dev/foundation/std/go/grpc/gateway"
	httpNode    schema.PackageName = "namespacelabs.dev/foundation/std/go/http"
)

func Register() {
	languages.Register(schema.Framework_GO_GRPC, impl{})
	runtime.RegisterSupport(schema.Framework_GO_GRPC, impl{})

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

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, _ *schema.Definition, x *OpGenServer) (*ops.HandleResult, error) {
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

func (impl) PrepareBuild(ctx context.Context, _ languages.Endpoints, server provision.Server, isFocus bool) (build.Spec, error) {
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

func (impl) TidyServer(ctx context.Context, pkgs workspace.Packages, loc workspace.Location, server *schema.Server) error {
	ext := &FrameworkExt{}
	if err := workspace.MustExtension(server.Ext, ext); err != nil {
		return fnerrors.Wrap(loc, err)
	}

	sdk, err := golang.MatchSDK(ext.GoVersion, golang.HostPlatform())
	if err != nil {
		return err
	}

	localSDK, err := compute.GetValue(ctx, sdk)
	if err != nil {
		return err
	}

	for _, dep := range loc.Module.Workspace.Dep {
		if dep.ModuleName == "namespacelabs.dev/foundation" {
			if err := execGo(ctx, loc, localSDK, "get", "-u", fmt.Sprintf("%s@%s", dep.ModuleName, dep.Version)); err != nil {
				return err
			}
		}
	}

	return execGo(ctx, loc, localSDK, "mod", "tidy")
}

func execGo(ctx context.Context, loc workspace.Location, sdk golang.LocalSDK, args ...string) error {
	return tasks.Action("go.run").HumanReadablef("go "+strings.Join(args, " ")).Run(ctx, func(ctx context.Context) error {
		var cmd localexec.Command
		cmd.Command = sdk.GoBin()
		cmd.Args = args
		cmd.AdditionalEnv = []string{sdk.GoRootEnv(), goPrivate()}
		cmd.Dir = loc.Abs()
		cmd.Label = "go " + strings.Join(cmd.Args, " ")
		return cmd.Run(ctx)
	})
}

func (impl) GenerateNode(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.Definition, error) {
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
		dl.Add("Generate Go proto sources", &source.OpProtoGen{
			PackageName:         pkg.PackageName().String(),
			GenerateHttpGateway: pkg.Node().ExportServicesAsHttp,
			Protos:              protos.Merge(list...),
			Framework:           source.OpProtoGen_GO,
		})
	}

	return dl.Serialize()
}

func (impl) GenerateServer(pkg *workspace.Package, nodes []*schema.Node) ([]*schema.Definition, error) {
	var dl defs.DefList
	dl.Add("Generate Go server dependencies", &OpGenServer{Server: pkg.Server, LoadedNode: nodes}, pkg.PackageName())
	return dl.Serialize()
}

func (impl) ParseNode(ctx context.Context, loc workspace.Location, _ *schema.Node, ext *workspace.FrameworkExt) error {
	return nil
}

func (impl) PreParseServer(ctx context.Context, loc workspace.Location, ext *workspace.FrameworkExt) error {
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

	if needGatewayCount > 0 && !sealed.HasDep(gatewayNode) {
		return fnerrors.UserError(sealed.Location, "server exposes gRPC services as HTTP, it must depend on %s", gatewayNode)
	}

	if len(sealed.Proto.Server.UrlMap) > 0 && !sealed.HasDep(httpNode) {
		return fnerrors.UserError(sealed.Location, "server exposes HTTP paths, it must depend on %s", httpNode)
	}

	return nil
}

func (impl) InjectService(loc workspace.Location, node *schema.Node, svc *workspace.CueService) error {
	pkg, err := packageFrom(loc)
	if err != nil {
		return err
	}

	svc.GoPackage = pkg
	return nil
}

func (impl) DevelopmentPackages() []schema.PackageName {
	return nil
}

func (impl) EvalProvision(*schema.Node) (frontend.ProvisionStack, error) {
	return frontend.ProvisionStack{}, nil
}

func packageFrom(loc workspace.Location) (string, error) {
	return gosupport.ComputeGoPackage(loc.Abs())
}

func (impl) FillEndpoint(n *schema.Node, e *schema.Endpoint) error {
	for _, exported := range n.ExportService {
		details, err := anypb.New(exported)
		if err != nil {
			return err
		}
		e.ServiceMetadata = append(e.ServiceMetadata, &schema.ServiceMetadata{
			Kind:     exported.ProtoTypename,
			Protocol: schema.GrpcProtocol,
			Details:  details,
		})

		if n.ExportServicesAsHttp {
			details, err := anypb.New(&schema.GrpcExportService{ProtoTypename: exported.ProtoTypename})
			if err != nil {
				return err
			}

			e.ServiceMetadata = append(e.ServiceMetadata, &schema.ServiceMetadata{
				Kind:    runtime.KindNeedsGrpcGateway,
				Details: details,
			})
		}
	}

	// Don't set protocol to avoid confusing ingress computation.
	// XXX this is now deprecated.
	metadata, err := toServiceMetadata([][2]string{{"/metrics", "prometheus.io/metrics"}}, "")
	if err != nil {
		return err
	}

	e.ServiceMetadata = append(e.ServiceMetadata, metadata...)
	return nil
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
