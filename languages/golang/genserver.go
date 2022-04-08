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
	allDeps, err := expandInstancedDeps(ctx, loader, srv.GetImportedPackages(), nodes)
	if err != nil {
		return err
	}

	opts.Server = "server"
	opts.Imports = gosupport.NewGoImports(loc.PackageName.String())

	opts.Imports.AddOrGet("namespacelabs.dev/foundation/std/go/grpc/server")
	opts.Imports.AddOrGet("namespacelabs.dev/foundation/std/go/core/init")

	usedNames := map[string]bool{"deps": true}

	// XXX use allocation tree instead.
	for _, dep := range allDeps.instances {
		// Force each of the type URLs to be known, so we do a single template pass.
		opts.Imports.AddOrGet(dep.Provisioned.GoPackage)

		var n *nodeWithDeps
		typename := "SingletonDeps"
		if dep.Scope != nil {
			typename = makeProvidesDepsType(dep.Scope)
		} else if dep.Parent.GetKind() == schema.Node_SERVICE {
			typename = "ServiceDeps"
		}
		for _, node := range opts.Nodes {
			if node.PackageName.Equals(dep.Parent.PackageName) &&
				node.Typename == typename {
				n = node
			}
		}

		if n == nil {
			n = &nodeWithDeps{
				Name:        makeName(filepath.Base(dep.Location.Rel()), usedNames, false), // XXX use package instead?
				PackageName: dep.Location.PackageName,
				Typename:    typename,
			}

			importURL, err := packageFrom(dep.Location)
			if err != nil {
				return err
			}

			n.GoImportURL = importURL

			if dep.Parent.GetKind() == schema.Node_SERVICE {
				n.VarName = fmt.Sprintf("%sDeps", n.Name)
				n.IsService = true

				if dep.Parent.ExportServicesAsHttp {
					for _, svc := range dep.Parent.ExportService {
						n.GrpcGatewayServices = append(n.GrpcGatewayServices, string(protoreflect.FullName(svc.ProtoTypename).Name()))
					}
				}

				opts.Services = append(opts.Services, n)
			} else if dep.Scope != nil {
				n.VarName = makeProvidesDepsVar(dep.Scope)
			} else {
				n.VarName = "singletonDeps"
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
				i.Deps = Ref{
					VarName:     node.VarName,
					GoImportURL: node.GoImportURL,
					Typename:    node.Typename,
				}
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
				Typename:    "ServiceDeps",
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
		n.Refs = make([][]Ref, len(n.Provisioned))
		for k, p := range n.Provisioned {
			switch len(p.Args) {
			case 0:
				for _, m := range opts.Nodes {
					if p.GoPackage == m.GoImportURL {
						ref := Ref{
							VarName:     m.VarName,
							Typename:    m.Typename,
							GoImportURL: m.GoImportURL,
						}
						if m.Typename == "SingletonDeps" || m.Typename == "ServiceDeps" {
							ref.IsSingleton = true
							n.Refs[k] = append([]Ref{ref}, n.Refs[k]...)
						} else if m.Typename == makeProvidesDepsType(p.Provides) {
							n.Refs[k] = append(n.Refs[k], ref)
						}
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
	Deps        Ref
}

type Ref struct {
	VarName     string
	GoImportURL string
	Typename    string
	IsSingleton bool
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
	Refs                [][]Ref // Same indexing as `Provisioned`.
}

type serverTmplOptions struct {
	Imports      *gosupport.GoImports
	Nodes        []*nodeWithDeps
	Services     []*nodeWithDeps
	Initializers []initializer
	Server       string
	PackageName  string
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
	{{$v.Name}} *{{$opts.Imports.MustGet $v.GoImportURL}}.{{$v.Typename}}{{end}}
}

func PrepareDeps(ctx context.Context) ({{$opts.Server}} *ServerDeps, err error) {
	di := {{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core/init"}}.MakeInitializer()
	{{range $k, $v := .Nodes}}
		di.Add({{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core/init"}}.Factory{
			PackageName: "{{$v.PackageName}}",
			Typename: "{{$v.Typename}}",
			Do: func(ctx context.Context, cf *{{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core/init"}}.CallerFactory) (interface{}, error) {
				deps := &{{makeType $opts.Imports $v.GoImportURL $v.Typename}}{}
				var err error
				var caller {{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core/init"}}.Caller
				{{- range $k2, $p := $v.Provisioned}}
					{{if $p -}}
							{
								{{ if $p.SerializedMsg -}}
								{{$p.ProtoComments -}}
								p := &{{$opts.Imports.MustGet $p.GoPackage}}.{{makeProvisionProtoName $p}}{}
								{{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core/init"}}.MustUnwrapProto("{{$p.SerializedMsg}}", p)

								{{end -}}

								{{range $p.DepVars -}}
								caller = cf.MakeCaller("{{.GoName}}")
								{{with $refs := index $v.Refs $k2}}{{range $k, $ref := $refs -}}
									{{$ref.VarName}}, err := di.Get{{if $ref.IsSingleton}}Singleton{{end}}(ctx, caller, "{{$p.PackageName}}", "{{$ref.Typename}}")
									if err != nil {
										return nil, err
									}
								{{end}}{{end -}}
								if deps.{{.GoName}}, err = {{$opts.Imports.MustGet $p.GoPackage}}.{{$p.Method}}(ctx, caller,
									{{- if $p.SerializedMsg}}p{{else}}nil{{end}}
									{{- with $refs := index $v.Refs $k2}}{{range $k, $ref := $refs}},{{$ref.VarName}}.(*{{makeType $opts.Imports $ref.GoImportURL $ref.Typename}}){{end}}{{end -}}
									); err != nil {
									return nil, err
								}
								{{- end}}
								{{range $kdep, $dep := $p.Dependencies}}
								{{with $depvar := index .DepVars 0}}
								deps.{{$depvar.GoName}}={{$opts.Imports.MustGet $dep.GoPackage}}.{{$dep.Method}}(deps.{{join $dep.Args ","}})
								{{end -}}
								{{end -}}
							}
					{{- end}}
				{{end -}}
				return deps, err
			},
		})
	{{end}}

	{{- range $k, $init := .Initializers}}
	di.AddInitializer({{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core/init"}}.Initializer{
		PackageName: "{{$init.PackageName}}",
		Do: func(ctx context.Context) error {
			{{- if $init.Deps.VarName}}
			{{$init.Deps.VarName}}, err := di.GetSingleton(ctx, "{{$init.PackageName}}", "{{$init.Deps.Typename}}")
			if err != nil {
				return err
			}
			{{end -}}
			return {{$opts.Imports.MustGet .GoImportURL}}.Prepare(ctx{{if $init.Deps.VarName}}, {{$init.Deps.VarName}}.(*{{makeType $opts.Imports $init.Deps.GoImportURL $init.Deps.Typename}}){{end}})
		},
	})
	{{end}}

	{{$opts.Server}} = &ServerDeps{}
	{{range $k, $v := .Services}}
		{{$v.VarName}}, err := di.GetSingleton(ctx, "{{$v.PackageName}}", "{{$v.Typename}}")
		if err != nil {
			return nil, err
		}
		{{$opts.Server}}.{{$v.Name}} = {{$v.VarName}}.(*{{makeType $opts.Imports $v.GoImportURL $v.Typename}})
	{{end}}
	return {{$opts.Server}}, di.Init(ctx)
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
