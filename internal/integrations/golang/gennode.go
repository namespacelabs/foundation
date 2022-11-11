// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"context"
	"strings"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/gosupport"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	depsFilename      = "deps.fn.go"
	extensionDepsType = "ExtensionDeps"
	serviceDepsType   = "ServiceDeps"
)

func generateNode(ctx context.Context, loader pkggraph.PackageLoader, loc pkggraph.Location, n *schema.Node, nodes []*schema.Node, fs fnfs.ReadWriteFS) error {
	gopkg, err := packageFrom(loc)
	if err != nil {
		return err
	}

	var g genTmplOptions
	g.Imports = gosupport.NewGoImports(gopkg)
	g.Imports.Ensure("context")

	pkgs := []schema.PackageName{loc.PackageName}
	pkgs = append(pkgs, n.GetImportedPackages()...)

	if err := prepareGenerate(ctx, loader, pkgs, &g); err != nil {
		return err
	}

	var providers []*nodeWithDeps
	for _, n := range g.Nodes {
		if n.PackageName == loc.PackageName {
			providers = append(providers, n)
		}
	}

	var initializers []goInitializer
	for _, n := range g.Initializers {
		if n.PackageName == loc.PackageName {
			initializers = append(initializers, n)
		}
	}

	var e instancedDepList
	if err := expandNode(ctx, loader, loc, n, false, &e); err != nil {
		return err
	}

	imports := g.Imports

	typ := "extension"
	single := &depsType{
		DepsType: extensionDepsType,
	}
	if n.GetKind() == schema.Node_SERVICE {
		typ = "service"
		single.DepsType = serviceDepsType
	}
	for _, dep := range e.instances {
		if dep.Scope == nil {
			single.DepVars = append(single.DepVars, dep.Provisioned.DepVars...)
		}
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
	if n.ServiceFramework == schema.Framework_FRAMEWORK_UNSPECIFIED &&
		len(n.ExportService) == 0 && len(n.ExportHttp) == 0 && !hasInitialization && providesCount == 0 {
		return nil
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
			for _, dep := range e.instances {
				if dep.Parent.PackageName == n.PackageName && prov.Name == dep.Scope.GetName() {
					s.DepVars = append(s.DepVars, dep.Provisioned.DepVars...)
				}
			}

			provides = append(provides, p)
			scoped = append(scoped, s)
			break
		}
	}

	if len(initializers) > 0 && len(single.DepVars) == 0 {
		// TODO remove with #717
		return fnerrors.New("%s: Nodes with initializers but no dependencies are not yet supported.", loc.PackageName)
	}

	return generateGoSource(ctx, fs, loc.Rel(depsFilename), imports, serviceTmpl, nodeTmplOptions{
		Type:           typ,
		Singleton:      single,
		PackageName:    loc.PackageName,
		Imports:        imports,
		Provides:       provides,
		Scoped:         scoped,
		NeedsSingleton: len(n.ExportService) > 0 || len(n.ExportHttp) > 0 || len(single.DepVars) > 0,
		Providers:      providers,
		Initializers:   initializers,
	})
}

func goPackageOr(loc pkggraph.Location, goPackage string) (string, error) {
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
	Type           string
	Singleton      *depsType
	PackageName    schema.PackageName
	Imports        *gosupport.GoImports
	Provides       []*typeProvider
	Scoped         []*depsType // Same indexing as `Provisioned`.
	NeedsSingleton bool

	Providers    []*nodeWithDeps
	Initializers []goInitializer
}

var (
	funcs = template.FuncMap{
		"join": strings.Join,

		"makeType": gosupport.MakeType,

		"makeProvisionProtoName": makeProvisionProtoName,

		"longPackageType": func(pkg schema.PackageName) string {
			return "Package__" + naming.StableIDN(pkg.String(), 6)
		},

		"longProviderType": func(pkg schema.PackageName, typ string) string {
			l := naming.StableIDN(pkg.String(), 6)
			if typ != "" {
				l += "__" + typ
			}
			return "Provider__" + l
		},

		"longInitializerType": func(pkg schema.PackageName) string {
			l := naming.StableIDN(pkg.String(), 6)
			return "Initializers__" + l
		},

		"longMakeDeps": func(pkg schema.PackageName, typ string) string {
			l := naming.StableIDN(pkg.String(), 6)
			if typ != "" {
				l += "__" + typ
			}
			return "makeDeps__" + l
		},
	}

	serviceTmpl = template.Must(template.New(depsFilename).Funcs(funcs).Parse(`{{with $opts := .}}
{{if .NeedsSingleton}}
// Dependencies that are instantiated once for the lifetime of the {{.Type}}.
type {{.Singleton.DepsType}} struct {
{{range $k, $v := .Singleton.DepVars}}
	{{$v.GoName}} {{$v.MakeType $opts.Imports}}{{end}}
}
{{end}}

{{range $k, $v := .Scoped}}
	{{if $v.DepVars}}
		// Scoped dependencies that are instantiated for each call to {{with $p := index $opts.Provides $k}}{{$p.Method}}{{end}}.
		type {{$v.DepsType}} struct {
		{{range $k, $i := $v.DepVars}}
			{{$i.GoName}} {{$i.MakeType $opts.Imports}}{{end}}
  		}
	{{end}}
{{end}}

{{if eq .Type "service"}}
// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, {{$opts.Imports.Ensure "namespacelabs.dev/foundation/std/go/server"}}Registrar, {{.Singleton.DepsType}})
var _ checkWireService = WireService
{{end}}

{{range $k, $v := .Provides}}
type _check{{$v.Method}} func(context.Context, *{{makeProvisionProtoName $v}}
	{{- if $opts.NeedsSingleton}}, {{$opts.Singleton.DepsType}}{{end}}
	{{- with $scoped := index $opts.Scoped $k}}{{if $scoped.DepVars}}, {{$scoped.DepsType}}{{end}}{{end -}}
	) ({{range $v.DepVars}}{{makeType $opts.Imports .GoImportURL .GoTypeName}},{{end}} error)
var _ _check{{$v.Method}} = {{$v.Method}}
{{end}}

var (
	{{longPackageType $opts.PackageName}} = &{{$opts.Imports.Ensure "namespacelabs.dev/foundation/std/go/core"}}Package{
		PackageName: "{{$opts.PackageName}}",
	}

{{range $k, $v := $opts.Providers -}}
	{{longProviderType $v.PackageName $v.Scope}} = {{$opts.Imports.Ensure "namespacelabs.dev/foundation/std/go/core"}}Provider{
		Package: {{longPackageType $opts.PackageName}},
		{{- if $v.Scope}}
		Typename: "{{$v.Scope}}",{{end}}
		Instantiate: {{longMakeDeps $v.PackageName $v.Scope}},
	}
{{end}}

{{if $opts.Initializers -}}
{{longInitializerType $opts.PackageName}} = []*{{$opts.Imports.Ensure "namespacelabs.dev/foundation/std/go/core"}}Initializer{
{{range $k, $init := .Initializers}} {
		Package: {{longPackageType $opts.PackageName}},{{if $init.InitializeBefore}}
		Before: []string{ {{range $init.InitializeBefore}}"{{.}}",{{end}}  },{{end}} {{if $init.InitializeAfter}}
		After: []string{ {{range $init.InitializeAfter}}"{{.}}",{{end}}  },{{end}}
	Do: func(ctx context.Context, di {{$opts.Imports.Ensure "namespacelabs.dev/foundation/std/go/core"}}Dependencies) error {
		{{- if $init.Deps}}
		return di.Instantiate(ctx, {{$opts.Imports.Ensure $init.GoImportURL}}{{longProviderType $init.PackageName ""}}, func(ctx context.Context, v interface{}) error {
		{{end -}}
		return {{$opts.Imports.Ensure .GoImportURL}}Prepare(ctx
			{{- if $init.Deps}}, v.({{makeType $opts.Imports $init.Deps.GoImportURL $init.Deps.Typename}}){{end -}}
		)
		{{- if $init.Deps}}
			})
		{{end -}}
	},
},
{{end}}
}
{{end}}
)

{{range $k, $v := $opts.Providers}}
func {{longMakeDeps $v.PackageName $v.Scope}}(ctx context.Context, di {{$opts.Imports.Ensure "namespacelabs.dev/foundation/std/go/core"}}Dependencies) (_ interface{}, err error) {
	var deps {{makeType $opts.Imports $v.GoImportURL $v.Typename}}
	{{if $v.Provisioned -}}
	{{- range $k2, $p := $v.Provisioned}}
		{{if $p -}}
			{{with $refs := index $v.Refs $k2}}
				{{- if $refs.Single}}
				if err := di.Instantiate(ctx, {{$opts.Imports.Ensure $p.PackageName.String}}{{longProviderType $p.PackageName ""}}, func(ctx context.Context, v interface{}) (err error) {
				{{- end}}
				{{- if $refs.Scoped}}
					{{- if $refs.Single}}return {{else}}
						if err := {{end -}}
				di.Instantiate(ctx, {{$opts.Imports.Ensure $p.PackageName.String}}{{longProviderType $p.PackageName $refs.Scoped.Scope}}, func(ctx context.Context, scoped interface{}) (err error) {
				{{end}}
				{{$p.ProtoComments -}}
				{{range $p.DepVars -}}
				if deps.{{.GoName}}, err = {{$opts.Imports.Ensure $p.GoPackage}}{{$p.Method}}(ctx,
					{{- if $p.SerializedMsg}} {{$opts.Imports.Ensure "namespacelabs.dev/foundation/std/go/core"}}MustUnwrapProto("{{$p.SerializedMsg}}", &{{$opts.Imports.Ensure $p.GoPackage}}{{makeProvisionProtoName $p}}{}).(*{{$opts.Imports.Ensure $p.GoPackage}}{{makeProvisionProtoName $p}})  {{else}}nil{{end -}}
					{{if $refs.Single}}, v.({{makeType $opts.Imports $refs.Single.GoImportURL $refs.Single.Typename}}){{end -}}
					{{if $refs.Scoped}}, scoped.({{makeType $opts.Imports $refs.Scoped.GoImportURL $refs.Scoped.Typename}}){{end -}}
					); err != nil {
					return {{if or $refs.Single $refs.Scoped}}err{{else}}nil, err{{end}}
				}
				{{- end}}
				{{- range $kdep, $dep := $p.Dependencies}}
					{{with $depvar := index .DepVars 0}}
					deps.{{$depvar.GoName}}={{$opts.Imports.Ensure $dep.GoPackage}}{{$dep.Method}}(deps.{{join $dep.Args ","}})
					{{end -}}
				{{end}}
				{{if or $refs.Single $refs.Scoped}}return nil{{end}}
				{{- if $refs.Scoped}}
					}) {{- if not $refs.Single -}} ; err != nil {
						return nil, err
					} {{end -}}
				{{end -}}
				{{- if $refs.Single}}
					}) ; err != nil {
						return nil, err
					}
				{{end}}
			{{end -}}
		{{end -}}
	{{end}}{{end}}
	return deps, nil
}
{{end}}

{{end}}`))
)
