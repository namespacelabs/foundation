// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"context"
	"path/filepath"
	"strings"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/gosupport"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const ServiceDepsFilename = "deps.fn.go"
const grpcServerPackage = "namespacelabs.dev/foundation/std/go/grpc/server"

func generateNode(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, nodes []*schema.Node, fs fnfs.ReadWriteFS) error {
	var e instancedDepList
	if err := expandNode(ctx, loader, loc, n, false, &e); err != nil {
		return err
	}

	var depVars []gosupport.TypeDef
	for _, p := range e.instances {
		depVars = append(depVars, p.Provisioned.DepVars...)
	}

	hasInitialization := n.InitializerFor(schema.Framework_GO) != nil
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

	imports := gosupport.NewGoImports(loc.PackageName.String())

	var depPackages []string
	if err := visitAllDeps(ctx, nodes, n.GetImportedPackages(), func(dep *schema.Node) error {
		if dep.InitializerFor(schema.Framework_GO) != nil {
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
	for _, dv := range depVars {
		if dv.GoImportURL != "" {
			imports.AddOrGet(dv.GoImportURL)
		}
	}

	typ := "Extension"
	if n.GetKind() == schema.Node_SERVICE {
		typ = "Service"

		imports.AddOrGet(grpcServerPackage)
	}

	var provides []*typeProvider
	for _, prov := range n.Provides {
		for _, available := range prov.AvailableIn {
			if available.Go == nil {
				continue
			}

			goImport, err := goPackageOr(loc, available.Go.Package)
			if err != nil {
				return err
			}

			imports.AddOrGet(goImport)

			provides = append(provides, &typeProvider{
				Provides:    prov,
				PackageName: loc.PackageName,
				Method:      makeProvidesMethod(prov),
				DepVars: []gosupport.TypeDef{{
					GoImportURL: goImport,
					GoTypeName:  available.Go.Type,
				}},
			})
			break
		}
	}

	return generateGoSource(ctx, fs, loc.Rel(ServiceDepsFilename), serviceTmpl, nodeTmplOptions{
		Type:              typ,
		PackageName:       filepath.Base(loc.Rel()),
		Imports:           imports,
		DepVars:           depVars,
		Provides:          provides,
		DepPackages:       depPackages,
		HasInitialization: hasInitialization,
		NeedsDepsType:     len(n.ExportService) > 0 || len(n.ExportHttp) > 0 || len(depVars) > 0,
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

type nodeTmplOptions struct {
	Type              string
	PackageName       string
	Imports           *gosupport.GoImports
	DepVars           []gosupport.TypeDef
	Provides          []*typeProvider
	DepPackages       []string
	HasInitialization bool
	NeedsDepsType     bool
}

var (
	funcs = template.FuncMap{
		"join": strings.Join,

		"makeType": gosupport.MakeType,

		"makeProvisionProtoName": makeProvisionProtoName,
	}

	serviceTmpl = template.Must(template.New(ServiceDepsFilename).Funcs(funcs).Parse(`// This file was automatically generated.{{with $opts := .}}
package {{$opts.PackageName}}

import (
	"context"

	{{range $opts.DepPackages}}{{if not ($opts.Imports.Has .)}}_ "{{.}}"{{end}}
	{{end}}

	{{range $opts.Imports.ImportMap}}
	{{.Rename}} "{{.TypeURL}}"{{end}}
)

{{if .NeedsDepsType}}
type {{.Type}}Deps struct {
{{range $k, $v := .DepVars}}
	{{$v.GoName}} {{$v.MakeType $opts.Imports}}{{end}}
}
{{end}}

{{if eq .Type "Service"}}
// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, *{{$opts.Imports.MustGet "namespacelabs.dev/foundation/std/go/grpc/server"}}.Grpc, {{.Type}}Deps)
var _ checkWireService = WireService
{{end}}

{{range $k, $v := .Provides}}
type _check{{$v.Method}} func(context.Context, string, *{{makeProvisionProtoName $v}}{{if $opts.DepVars}}, {{$opts.Type}}Deps{{end}}) ({{range $v.DepVars}}{{makeType $opts.Imports .GoImportURL .GoTypeName}},{{end}} error)

var _ _check{{$v.Method}} = {{$v.Method}}
{{end}}

{{if .HasInitialization}}
type _checkPrepare func(context.Context{{if .NeedsDepsType}}, {{.Type}}Deps{{end}}) error
var _ _checkPrepare = Prepare
{{end}}

{{end}}`))
)
