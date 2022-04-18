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

func generateServer(ctx context.Context, loader workspace.Packages, loc workspace.Location, srv *schema.Server, fs fnfs.ReadWriteFS) error {
	var opts genTmplOptions

	opts.Imports = gosupport.NewGoImports("main")
	opts.Imports.Ensure("context")

	if err := prepareGenerate(ctx, loader, srv.GetImportedPackages(), &opts); err != nil {
		return err
	}

	if err := generateGoSource(ctx, fs, loc.Rel(ServerPrepareFilename), opts.Imports, serverPrepareTmpl, opts); err != nil {
		return err
	}

	if err := generateGoSource(ctx, fs, loc.Rel(ServerMainFilename), nil, mainTmpl, mainTmplOptions{
		PackageName: srv.GetPackageName(),
	}); err != nil {
		return err
	}

	return nil
}

func prepareGenerate(ctx context.Context, loader workspace.Packages, imports []schema.PackageName, opts *genTmplOptions) error {
	allDeps, err := expandInstancedDeps(ctx, loader, imports)
	if err != nil {
		return err
	}

	// Prepopulate variable names that are used in serverPrepareTmpl.
	usedNames := map[string]bool{
		"deps": true,
		"di":   true,
		"err":  true,
	}

	// XXX use allocation tree instead.
	for _, dep := range allDeps.instances {
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
	}

	for _, init := range allDeps.initializers {
		pkg, err := packageFrom(init.Location)
		if err != nil {
			return err
		}

		i := initializer{
			PackageName: schema.PackageName(init.Node.PackageName),
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
			var n *nodeWithDeps
			for _, node := range opts.Nodes {
				if node.PackageName == svc.Location.PackageName {
					n = node
				}
			}
			if n == nil {
				// Ensure that services with no deps create an empty provider.
				n = &nodeWithDeps{
					Name:        makeName(filepath.Base(svc.Location.Rel()), usedNames, false), // XXX use package instead?
					VarName:     makeName(filepath.Base(svc.Location.Rel()), usedNames, true),
					PackageName: svc.Location.PackageName,
					Typename:    serviceDepsType,
					IsService:   true,
				}
				opts.Nodes = append(opts.Nodes, n)
			}

			importURL, err := packageFrom(svc.Location)
			if err != nil {
				return err
			}

			n.GoImportURL = importURL
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
	PackageName schema.PackageName
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

type genTmplOptions struct {
	Imports      *gosupport.GoImports
	Nodes        []*nodeWithDeps
	Services     []*nodeWithDeps
	Initializers []initializer
}

type mainTmplOptions struct {
	PackageName string
}

var (
	serverPrepareTmpl = template.Must(template.New(ServerPrepareFilename).Funcs(funcs).Parse(`{{with $opts := .}}
func RegisterInitializers(di *{{$opts.Imports.Ensure "namespacelabs.dev/foundation/std/go/core"}}DependencyGraph) {
	{{- range $k, $init := .Initializers}}
	di.AddInitializers({{$opts.Imports.Ensure $init.GoImportURL}}{{longInitializerType $init.PackageName}}...)
	{{- end}}
}

func WireServices(ctx context.Context, srv {{$opts.Imports.Ensure "namespacelabs.dev/foundation/std/go/server"}}Server, depgraph {{$opts.Imports.Ensure "namespacelabs.dev/foundation/std/go/core"}}Dependencies) []error {
	var errs []error
{{range $k, $v := .Services}}
	if err := depgraph.Instantiate(ctx, {{$opts.Imports.Ensure $v.PackageName.String}}{{longProviderType $v.PackageName ""}}, func(ctx context.Context, v interface{}) error {
			{{$opts.Imports.Ensure $v.GoImportURL}}WireService(ctx, srv.Scope({{$opts.Imports.Ensure $v.PackageName.String}}{{longPackageType $v.PackageName}}), v.({{makeType $opts.Imports $v.GoImportURL $v.Typename}}))
			return nil
		}); err != nil{
			errs = append(errs, err)
		}

{{range $v.GrpcGatewayServices}}srv.InternalRegisterGrpcGateway({{$opts.Imports.Ensure $v.GoImportURL}}Register{{.}}Handler)
{{end -}}
{{end}}
	return errs
}
{{end}}`))

	mainTmpl = template.Must(template.New(ServerMainFilename).Parse(`// This file was automatically generated.
package main

import (
	"context"
	"flag"
	"log"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/server"
)

func main() {
	flag.Parse()

	resources := core.PrepareEnv("{{.PackageName}}")
	defer resources.Close(context.Background())

	ctx := core.WithResources(context.Background(), resources)

	depgraph := core.NewDependencyGraph()
	RegisterInitializers(depgraph)
	if err := depgraph.RunInitializers(ctx); err != nil {
		log.Fatal(err)
	}

	server.InitializationDone()

	server.Listen(ctx, func(srv server.Server) {
		if errs := WireServices(ctx, srv, depgraph); len(errs) > 0 {
			for _, err := range errs {
				log.Println(err)
			}
			log.Fatalf("%d services failed to initialize.", len(errs))
		}
	})
}
`))
)
