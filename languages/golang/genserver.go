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
	opts.Imports.AddOrGet("namespacelabs.dev/foundation/std/go/core")

	// Prepopulate variable names that are used in serverPrepareTmpl.
	usedNames := map[string]bool{
		"caller": true,
		"deps":   true,
		"di":     true,
		"err":    true,
	}

	// XXX use allocation tree instead.
	for _, dep := range allDeps.instances {
		// Force each of the type URLs to be known, so we do a single template pass.
		opts.Imports.AddOrGet(dep.Provisioned.GoPackage)

		var n *nodeWithDeps
		var scope string
		if dep.Scope != nil {
			scope = gosupport.MakeGoPubVar(dep.Scope.Name)
		}
		for _, node := range opts.Nodes {
			if node.PackageName.Equals(dep.Parent.PackageName) &&
				node.Scope == scope {
				n = node
			}
		}

		if n == nil {
			n = &nodeWithDeps{
				Name:        makeName(filepath.Base(dep.Location.Rel()), usedNames, false), // XXX use package instead?
				PackageName: dep.Location.PackageName,
				Scope:       scope,
			}

			importURL, err := packageFrom(dep.Location)
			if err != nil {
				return err
			}

			n.GoImportURL = importURL

			if dep.Parent.GetKind() == schema.Node_SERVICE {
				n.VarName = fmt.Sprintf("%sDeps", n.Name)
				n.IsService = true
				n.Typename = serviceDepsType

				if dep.Parent.ExportServicesAsHttp {
					for _, svc := range dep.Parent.ExportService {
						n.GrpcGatewayServices = append(n.GrpcGatewayServices, string(protoreflect.FullName(svc.ProtoTypename).Name()))
					}
				}

				opts.Services = append(opts.Services, n)
			} else if dep.Scope != nil {
				n.Typename = makeProvidesDepsType(dep.Scope)
			} else {
				n.Typename = extensionDepsType
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
				Typename:    serviceDepsType,
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
		n.Refs = make([]Refs, len(n.Provisioned))
		for k, p := range n.Provisioned {
			switch len(p.Args) {
			case 0:
				for _, m := range opts.Nodes {
					if p.GoPackage == m.GoImportURL {
						ref := Ref{
							Typename:    m.Typename,
							GoImportURL: m.GoImportURL,
							Scope:       m.Scope,
						}
						if m.Scope == "" {
							n.Refs[k].Single = &ref
						} else if m.Typename == makeProvidesDepsType(p.Provides) {
							n.Refs[k].Scoped = &ref
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
	GoImportURL string
	Scope       string
	Typename    string
}

type Refs struct {
	Single *Ref
	Scoped *Ref
}

type nodeWithDeps struct {
	goPackage
	PackageName         schema.PackageName
	Name                string
	VarName             string
	Typename            string
	Scope               string
	IsService           bool
	GrpcGatewayServices []string
	Provisioned         []*typeProvider
	Refs                []Refs // Same indexing as `Provisioned`.
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
	{{$v.Name}} {{$opts.Imports.MustGet $v.GoImportURL}}ServiceDeps{{end}}
}

// This code uses type assertions for now. When go 1.18 is more common, it will switch to generics.
func PrepareDeps(ctx context.Context) ({{$opts.Server}} *ServerDeps, err error) {
	di := {{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}MakeInitializer()
	{{range $k, $v := .Nodes}}
		di.Add({{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}Provider{
			Package: "{{$v.PackageName}}",
			{{- if $v.Scope}}
			Scope: "{{$v.Scope}}",{{end}}
			Do: func(ctx context.Context) (interface{}, error) {
				var deps {{makeType $opts.Imports $v.GoImportURL $v.Typename}}
				var err error
				{{- range $k2, $p := $v.Provisioned}}
					{{if $p -}}
						{{with $refs := index $v.Refs $k2}}
							{{- if and (not $refs.Single) (not $refs.Scoped) (gt (len $v.Provisioned) 1)}} {
							{{end}}
							{{- if $refs.Single}}
							err = di.Instantiate(ctx,
								{{- $opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}Reference{Package: "{{$p.PackageName}}"},
								func(ctx context.Context, v interface{}) (err error) {
							{{end}}
							{{- if $refs.Scoped}}
								{{- if $refs.Single}}return {{else}}
									err = {{end -}}
							di.Instantiate(ctx, {{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}Reference{
									Package: "{{$p.PackageName}}",
									Scope: "{{$refs.Scoped.Scope}}"},
								func(ctx context.Context, scoped interface{}) (err error) { 
							{{end -}}
							{{- if $p.SerializedMsg -}}
							{{$p.ProtoComments -}}
							p := &{{$opts.Imports.MustGet $p.GoPackage}}{{makeProvisionProtoName $p}}{}
							{{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}MustUnwrapProto("{{$p.SerializedMsg}}", p)

							{{end}}
							{{range $p.DepVars -}}
							if deps.{{.GoName}}, err = {{$opts.Imports.MustGet $p.GoPackage}}{{$p.Method}}(ctx,
								{{- if $p.SerializedMsg}}p{{else}}nil{{end -}}
								{{if $refs.Single}}, v.({{makeType $opts.Imports $refs.Single.GoImportURL $refs.Single.Typename}}){{end -}}
								{{if $refs.Scoped}}, scoped.({{makeType $opts.Imports $refs.Scoped.GoImportURL $refs.Scoped.Typename}}){{end -}}
								); err != nil {
								return {{if or $refs.Single $refs.Scoped}}err{{else}}nil, err{{end}}
							}
							{{- end}}
							{{- range $kdep, $dep := $p.Dependencies}}
								{{with $depvar := index .DepVars 0}}
								deps.{{$depvar.GoName}}={{$opts.Imports.MustGet $dep.GoPackage}}{{$dep.Method}}(deps.{{join $dep.Args ","}})
								{{end -}}
							{{end}}
							{{if or $refs.Single $refs.Scoped}}return nil{{end}}
							{{- if $refs.Scoped}}
								})
								{{- if not $refs.Single}}
								if err != nil {
									return nil, err
								} {{end -}}
							{{end -}}
							{{if $refs.Single}}
								})
								if err != nil {
									return nil, err
								}
							{{end}}
							{{- if and (not $refs.Single) (not $refs.Scoped) (gt (len $v.Provisioned) 1)}} } {{end}}
						{{end -}}
					{{end -}}
				{{end}}
				return deps, err
			},
		})
	{{end}}

	{{$opts.Server}} = &ServerDeps{}
	di.AddInitializer({{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}Initializer{
		PackageName: "{{$opts.PackageName}}",
		Do: func(ctx context.Context) error {
			{{range $k, $v := .Services}}
			err = di.Instantiate(ctx,
				{{- $opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}Reference{Package: "{{$v.PackageName}}"},
				func(ctx context.Context, v interface{}) (err error) {
					{{$opts.Server}}.{{$v.Name}} = v.({{makeType $opts.Imports $v.GoImportURL $v.Typename}})
					return nil
				})
			if err != nil {
				return err
			}
			{{end}}
			return nil
		},
	})

	{{- range $k, $init := .Initializers}}
	di.AddInitializer({{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}Initializer{
		PackageName: "{{$init.PackageName}}",
		Do: func(ctx context.Context) error {
			{{- if $init.Deps}}
			return di.Instantiate(ctx,
				{{- $opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/core"}}Reference{Package: "{{$init.PackageName}}"},
				func(ctx context.Context, v interface{}) (err error) {
			{{end -}}
			return {{$opts.Imports.MustGet .GoImportURL}}Prepare(ctx
				{{- if $init.Deps}}, v.({{makeType $opts.Imports $init.Deps.GoImportURL $init.Deps.Typename}}){{end -}}
			)
			{{- if $init.Deps}}
				})
			{{end -}}
		},
	})
	{{end}}

	return {{$opts.Server}}, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *{{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/grpc/server"}}Grpc, server *ServerDeps) {
{{range $k, $v := .Services}}{{$opts.Imports.MustGet $v.GoImportURL}}WireService(ctx, srv, server.{{$v.Name}})
{{range $v.GrpcGatewayServices}}srv.RegisterGrpcGateway({{$opts.Imports.MustGet $v.GoImportURL}}Register{{.}}Handler)
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
