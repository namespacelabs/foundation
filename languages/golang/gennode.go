// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"context"
	"path/filepath"
	"strings"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/gosupport"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const depsFilename = "deps.fn.go"
const grpcServerPackage = "namespacelabs.dev/foundation/std/go/grpc/server"
const corePackage = "namespacelabs.dev/foundation/std/go/core"

func generateNode(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, nodes []*schema.Node, fs fnfs.ReadWriteFS) error {
	var e instancedDepList
	produceSerialized := false
	if err := expandNode(ctx, loader, loc, n, produceSerialized, &e); err != nil {
		return err
	}

	imports := gosupport.NewGoImports(loc.PackageName.String())

	typ := "Extension"
	single := &depsType{
		DepsType: "SingletonDeps",
	}
	if n.GetKind() == schema.Node_SERVICE {
		typ = "Service"
		single.DepsType = "ServiceDeps"

		imports.AddOrGet(grpcServerPackage)
	}
	for _, p := range e.instances {
		single.DepVars = append(single.DepVars, p.Provisioned.DepVars...)
	}

	hasInitialization := n.InitializerFor(schema.Framework_GO_GRPC) != nil
	var providesCount int
	for _, p := range n.Provides {
		for _, prov := range p.AvailableIn {
			if prov.Go != nil {
				providesCount++
			}
		}
	}

	// Irrespective of whether instances are declared, if there's no code to consume them, don't codegen.
	if len(n.ExportService) == 0 && len(n.ExportHttp) == 0 && !hasInitialization && providesCount == 0 {
		return nil
	}

	var depPackages []string
	if err := visitAllDeps(ctx, nodes, n.GetImportedPackages(), func(dep *schema.Node) error {
		if dep.InitializerFor(schema.Framework_GO_GRPC) != nil {
			nodeLoc, err := loader.Resolve(ctx, schema.PackageName(dep.PackageName))
			if err != nil {
				return err
			}
			pkg, err := packageFrom(nodeLoc)
			if err != nil {
				return err
			}
			depPackages = append(depPackages, pkg)
		}
		return nil
	}); err != nil {
		return err
	}

	// Force each of the type URLs to be known, so we do a single template pass.
	for _, dv := range single.DepVars {
		if dv.GoImportURL != "" {
			imports.AddOrGet(dv.GoImportURL)
		}
	}

	var provides []*typeProvider
	var scoped []*depsType
	for _, prov := range n.Provides {
		for _, available := range prov.AvailableIn {
			if available.Go == nil {
				continue
			}

			// Skip provides which have computable availableIn, i.e. empty blocks.
			if available.Go.GetType() == "" {
				continue
			}

			goImport, err := goPackageOr(loc, available.Go.Package)
			if err != nil {
				return err
			}

			imports.AddOrGet(goImport)

			p := &typeProvider{
				Provides:    prov,
				PackageName: loc.PackageName,
				Method:      makeProvidesMethod(prov),
				DepVars: []gosupport.TypeDef{{
					GoImportURL: goImport,
					GoTypeName:  available.Go.Type,
				}}}

			s := &depsType{
				DepsType: makeProvidesDepsType(prov),
			}
			for k, dep := range prov.Instantiate {
				var prov typeProvider

				if err := makeDep(ctx, loader, dep, produceSerialized, &prov); err != nil {
					return fnerrors.UserError(loc, "%s.dependency[%d]: %w", n.GetPackageName(), k, err)
				}
				s.DepVars = append(s.DepVars, prov.DepVars...)

				for _, dv := range prov.DepVars {
					if dv.GoImportURL != "" {
						imports.AddOrGet(dv.GoImportURL)
					}
				}
			}

			provides = append(provides, p)
			scoped = append(scoped, s)
			break
		}
	}

	return generateGoSource(ctx, fs, loc.Rel(depsFilename), serviceTmpl, nodeTmplOptions{
		Type:              typ,
		Singleton:         single,
		PackageName:       filepath.Base(loc.Rel()),
		Imports:           imports,
		Provides:          provides,
		Scoped:            scoped,
		DepPackages:       depPackages,
		HasInitialization: hasInitialization,
		NeedsSingleton:    len(n.ExportService) > 0 || len(n.ExportHttp) > 0 || len(single.DepVars) > 0,
	})
}

func goPackageOr(loc workspace.Location, goPackage string) (string, error) {
	if goPackage != "" {
		return goPackage, nil
	}
	return packageFrom(loc)
}

func makeProvisionProtoName(p *typeProvider) string {
	parts := strings.Split(p.Provides.Type.Typename, ".")
	return gosupport.MakeGoPubVar(parts[len(parts)-1])
}

type depsType struct {
	DepsType string
	DepVars  []gosupport.TypeDef
}

type nodeTmplOptions struct {
	Type              string
	Singleton         *depsType
	PackageName       string
	Imports           *gosupport.GoImports
	Provides          []*typeProvider
	Scoped            []*depsType // Same indexing as `Provisioned`.
	DepPackages       []string
	HasInitialization bool
	NeedsSingleton    bool
}

var (
	funcs = template.FuncMap{
		"join": strings.Join,

		"makeType": gosupport.MakeType,

		"makeProvisionProtoName": makeProvisionProtoName,
	}

	serviceTmpl = template.Must(template.New(depsFilename).Funcs(funcs).Parse(`// This file was automatically generated.{{with $opts := .}}
package {{$opts.PackageName}}

import (
	"context"

	{{range $opts.DepPackages}}{{if not ($opts.Imports.Has .)}}_ "{{.}}"{{end}}
	{{end}}

	{{range $opts.Imports.ImportMap}}
	{{.Rename}} "{{.TypeURL}}"{{end}}
)

{{if .NeedsSingleton}}
type {{.Singleton.DepsType}} struct {
{{range $k, $v := .Singleton.DepVars}}
	{{$v.GoName}} {{$v.MakeType $opts.Imports}}{{end}}
}
{{end}}

{{range $k, $v := .Scoped}}
	{{if $v.DepVars}}
		// Scoped dependencies that are reinstantiated for each call to {{with $p := index $opts.Provides $k}}{{$p.Method}}{{end}}
		type {{$v.DepsType}} struct {
		{{range $k, $i := $v.DepVars}}
			{{$i.GoName}} {{$i.MakeType $opts.Imports}}{{end}}
  		}
	{{end}}
{{end}}

{{if eq .Type "Service"}}
// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, *{{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/grpc/server"}}.Grpc, {{.Singleton.DepsType}})
var _ checkWireService = WireService
{{end}}

{{range $k, $v := .Provides}}
type _check{{$v.Method}} func(context.Context, string, *{{makeProvisionProtoName $v}}
	{{- if $opts.NeedsSingleton}}, {{$opts.Singleton.DepsType}}{{end}}
	{{- with $scoped := index $opts.Scoped $k}}{{if $scoped.DepVars}}, {{$scoped.DepsType}}{{end}}{{end -}}
	) ({{range $v.DepVars}}{{makeType $opts.Imports .GoImportURL .GoTypeName}},{{end}} error)
var _ _check{{$v.Method}} = {{$v.Method}}
{{end}}

{{if .HasInitialization}}
type _checkPrepare func(context.Context{{if .NeedsSingleton}}, {{.Singleton.DepsType}}{{end}}) error
var _ _checkPrepare = Prepare
{{end}}

{{end}}`))
)
