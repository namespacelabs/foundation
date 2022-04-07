// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/rs/zerolog"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/gosupport"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const (
	ServerPrepareFilename = "deps.fn.go"
	ServerMainFilename    = "main.fn.go"
)

func generateServer(ctx context.Context, loader workspace.Packages, loc workspace.Location, srv *schema.Server, nodes []*schema.Node, fs fnfs.ReadWriteFS) error {
	var opts serverTmplOptions

	if err := prepareServer(ctx, loader, loc, srv, nodes, &opts); err != nil {
		return err
	}

	if strings.Contains(srv.Name, "multidb") {
		for _, n := range opts.Nodes {
			zerolog.Ctx(ctx).Info().
				Stringer("n", n.PackageName).
				Str("var", n.VarName).
				Strs("refs", n.Refs).
				Msg("generateServer")
			for _, p := range n.Provisioned {
				zerolog.Ctx(ctx).Info().
					Stringer("prov", p.PackageName).
					Str("method", p.Method).
					Msg("generateServer")
			}
		}
	}

	if err := generateGoSource(ctx, fs, loc.Rel(ServerPrepareFilename), serverPrepareTmpl, opts); err != nil {
		return err
	}

	if err := generateGoSource(ctx, fs, loc.Rel(ServerMainFilename), mainTmpl, mainTmplOptions{
		PackageName: srv.GetPackageName(),
	}); err != nil {
		return err
	}

	return nil
}

func prepareServer(ctx context.Context, loader workspace.Packages, loc workspace.Location, srv *schema.Server, nodes []*schema.Node, opts *serverTmplOptions) error {
	debug := false
	if strings.Contains(srv.Name, "multidb") {
		debug = true
		for _, p := range srv.GetImportedPackages() {
			zerolog.Ctx(ctx).Info().
				Stringer("import", p).
				Msg("prepareServer")
		}
	}

	allDeps, err := expandInstancedDeps(ctx, loader, srv.GetImportedPackages(), nodes)
	if err != nil {
		return err
	}

	opts.Server = "server"
	opts.Imports = gosupport.NewGoImports(loc.PackageName.String())

	opts.Imports.AddOrGet("namespacelabs.dev/foundation/std/go/grpc/server")

	if len(allDeps.instances) > 0 {
		opts.Imports.AddOrGet("namespacelabs.dev/foundation/std/go/core")
	}

	usedNames := map[string]bool{}

	// XXX use allocation tree instead.
	for _, dep := range allDeps.instances {
		if debug {
			zerolog.Ctx(ctx).Info().Stringer("dep", dep.Location.PackageName).Msg("prepareServer")
		}
		// Force each of the type URLs to be known, so we do a single template pass.
		opts.Imports.AddOrGet(dep.Provisioned.GoPackage)

		var n *nodeWithDeps
		for _, node := range opts.Nodes {
			if node.PackageName.Equals(dep.Parent.PackageName) {
				n = node
			}
		}

		if n == nil {
			n = &nodeWithDeps{
				Name:        makeName(filepath.Base(dep.Location.Rel()), usedNames, false), // XXX use package instead?
				PackageName: dep.Location.PackageName,
			}

			importURL, err := packageFrom(dep.Location)
			if err != nil {
				return err
			}

			n.GoImportURL = importURL

			if dep.Parent.GetKind() == schema.Node_SERVICE {
				n.Typename = "ServiceDeps"
				n.VarName = fmt.Sprintf("%s.%s", opts.Server, n.Name)
				n.IsService = true

				if dep.Parent.ExportServicesAsHttp {
					for _, svc := range dep.Parent.ExportService {
						n.GrpcGatewayServices = append(n.GrpcGatewayServices, string(protoreflect.FullName(svc.ProtoTypename).Name()))
					}
				}

				opts.Services = append(opts.Services, n)
			} else {
				n.Typename = "SingletonDeps"
				n.VarName = makeName(filepath.Base(dep.Location.Rel()), usedNames, true)
			}

			opts.Nodes = append(opts.Nodes, n)
		}

		n.Provisioned = append(n.Provisioned, dep.Provisioned)

		opts.Imports.AddOrGet(n.GoImportURL)
	}

	for _, init := range allDeps.initializers {
		pkg, err := packageFrom(init.Location)
		if err != nil {
			return err
		}

		i := initializer{
			PackageName: init.Node.PackageName,
			GoImportURL: pkg,
		}

		for _, node := range opts.Nodes {
			if node.PackageName.Equals(init.Node.PackageName) {
				i.Deps = node.VarName
				break
			}
		}

		opts.Initializers = append(opts.Initializers, i)
		opts.Imports.AddOrGet(pkg)
	}

	for _, svc := range allDeps.services {
		if len(svc.Node.ExportService) == 0 && len(svc.Node.ExportHttp) == 0 {
			continue
		}

		has := false
		for _, existing := range opts.Services {
			pkg, err := packageFrom(svc.Location)
			if err != nil {
				return err
			}

			if existing.GoImportURL == pkg { // XXX this is not quite right.
				has = true
				break
			}
		}

		if !has {
			n := &nodeWithDeps{
				Name:        makeName(filepath.Base(svc.Location.Rel()), usedNames, false), // XXX use package instead?
				VarName:     makeName(filepath.Base(svc.Location.Rel()), usedNames, true),
				PackageName: svc.Location.PackageName,
				Typename:    "SingletonDeps",
				IsService:   true,
			}

			importURL, err := packageFrom(svc.Location)
			if err != nil {
				return err
			}

			n.GoImportURL = importURL
			opts.Imports.AddOrGet(n.GoImportURL)

			opts.Services = append(opts.Services, n)
		}
	}

	// XXX another O(n^2); and this is incorrect when there are multiple nodes
	// allocating the same instance types.
	for _, n := range opts.Nodes {
		n.Refs = make([]string, len(n.Provisioned))
		for k, p := range n.Provisioned {
			switch len(p.Args) {
			case 0:
				for _, m := range opts.Nodes {
					if p.GoPackage == m.GoImportURL {
						n.Refs[k] = m.VarName
						break
					}
				}
			case 1:
				found := false
				for _, x := range n.Provisioned {
					for _, name := range x.DepVars {
						if name.GoName == p.Args[0] {
							found = true
							x.Dependencies = append(x.Dependencies, p)
							n.Provisioned[k] = nil
							break
						}
					}
				}
				if !found {
					return fnerrors.UserError(nil, "didn't find reference: %s", p.Args[0])
				}
			default:
				return fnerrors.UserError(nil, "Instantiate: only support one reference right now, saw %d", len(p.Args))
			}
		}
	}

	return nil
}

func makeName(path string, m map[string]bool, withSuffix bool) string {
	svcBaseName := gosupport.MakeGoPrivVar(strings.Replace(path, "/", "_", -1))
	svcName := svcBaseName
	if withSuffix {
		svcName += "0"
	}
	k := 1
	for {
		if _, ok := m[svcName]; !ok {
			m[svcName] = true
			return svcName
		}

		svcName = fmt.Sprintf("%s%d", svcBaseName, k)
		k++
	}
}

type goPackage struct {
	GoImportURL string
}

type initializer struct {
	PackageName string
	GoImportURL string
	Deps        string
}

type nodeWithDeps struct {
	goPackage
	PackageName         schema.PackageName
	Name                string
	VarName             string
	Typename            string
	IsService           bool
	GrpcGatewayServices []string
	Provisioned         []*typeProvider
	Refs                []string // Same indexing as `Provisioned`.
}

type serverTmplOptions struct {
	Imports      *gosupport.GoImports
	Nodes        []*nodeWithDeps
	Services     []*nodeWithDeps
	Initializers []initializer
	Server       string
}

type mainTmplOptions struct {
	PackageName string
}

var (
	serverPrepareTmpl = template.Must(template.New(ServerPrepareFilename).Funcs(funcs).Parse(`// This file was automatically generated.{{with $opts := .}}
package main

import (
	"context"
{{range $opts.Imports.ImportMap}}
	{{.Rename}} "{{.TypeURL}}"{{end}}
)

type ServerDeps struct {
{{range $k, $v := .Services}}
	{{$v.Name}} {{$opts.Imports.MustGet $v.GoImportURL}}.{{$v.Typename}}{{end}}
}

func PrepareDeps(ctx context.Context) (*ServerDeps, error) {
	var {{$opts.Server}} ServerDeps
	var di {{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}.DepInitializer
	{{range $k, $v := .Nodes}}
		{{- if not $v.IsService}}var {{$v.VarName}} {{makeType $opts.Imports $v.GoImportURL $v.Typename}}{{end}}
		{{- range $k2, $p := $v.Provisioned}}
			{{if $p}}
				di.Register({{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}.Initializer{
					PackageName: "{{$p.PackageName}}",
					Instance: "{{$v.VarName}}",
					{{with $ref := index $v.Refs $k2}}{{if $ref}}DependsOn: []string{"{{$ref}}"},{{end}}{{end -}}
					Do: func(ctx context.Context) (err error) {
						{{if $p.SerializedMsg -}}
						{{$p.ProtoComments -}}
						p := &{{$opts.Imports.MustGet $p.GoPackage}}.{{makeProvisionProtoName $p}}{}
						{{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}.MustUnwrapProto("{{$p.SerializedMsg}}", p)

						{{end -}}

						{{range $p.DepVars -}}
						if {{$v.VarName}}.{{.GoName}}, err = {{$opts.Imports.MustGet $p.GoPackage}}.{{$p.Method}}(ctx, "{{$v.PackageName}}", {{if $p.SerializedMsg}}p{{else}}nil{{end}}{{with $ref := index $v.Refs $k2}}{{if $ref}}, {{$ref}}{{end}}{{end}}{{if $p.DepsType}}, deps{{end}}); err != nil {
							return err
						}
						{{- end}}
						{{range $kdep, $dep := $p.Dependencies}}
						{{with $depvar := index .DepVars 0}}
						{{$v.VarName}}.{{$depvar.GoName}}={{$opts.Imports.MustGet $dep.GoPackage}}.{{$dep.Method}}({{$v.VarName}}.{{join $dep.Args ","}})
						{{end -}}
						{{end -}}
						return nil
					},
				})
			{{end}}
		{{end}}
	{{end}}

	{{- range .Initializers}}
	di.Register({{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}.Initializer{
		PackageName: "{{.PackageName}}",
		{{if .Deps}}DependsOn: []string{"{{.Deps}}"},{{end}}
		Do: func(ctx context.Context) error {
			return {{$opts.Imports.MustGet .GoImportURL}}.Prepare(ctx{{if .Deps}}, {{.Deps}}{{end}})
		},
	})
	{{end}}

	return &{{$opts.Server}}, di.Wait(ctx)
}

func WireServices(ctx context.Context, srv *{{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/grpc/server"}}.Grpc, server *ServerDeps) {
{{range $k, $v := .Services}}{{$opts.Imports.MustGet $v.GoImportURL}}.WireService(ctx, srv, server.{{$v.Name}})
{{range $v.GrpcGatewayServices}}srv.RegisterGrpcGateway({{$opts.Imports.MustGet $v.GoImportURL}}.Register{{.}}Handler)
{{end -}}
{{end}}}
{{end}}`))

	mainTmpl = template.Must(template.New(ServerMainFilename).Parse(`// This file was automatically generated.
package main

import (
	"context"
	"flag"
	"log"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/server"
)

func main() {
	flag.Parse()

	resources := core.PrepareEnv("{{.PackageName}}")
	defer resources.Close(context.Background())

	ctx := core.WithResources(context.Background(), resources)

	deps, err := PrepareDeps(ctx)
	if err != nil {
		log.Fatal(err)
	}

	server.InitializationDone()

	server.ListenGRPC(ctx, func(srv *server.Grpc) {
		WireServices(ctx, srv, deps)
	})
}
`))
)
