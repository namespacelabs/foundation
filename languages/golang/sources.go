// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"context"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/slices"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func ComputeSources(ctx context.Context, root string, srv provision.Server, platforms []specs.Platform) (d *D, err error) {
	err = tasks.Action("go.compute-sources").Run(ctx, func(ctx context.Context) error {
		var err error
		d, err = computeSources(ctx, root, srv, platforms)
		return err
	})
	return
}

func computeSources(ctx context.Context, root string, srv provision.Server, platforms []specs.Platform) (*D, error) {
	spec, err := (impl{}).PrepareBuild(ctx, languages.AvailableBuildAssets{}, srv, false)
	if err != nil {
		return nil, err
	}

	bin := spec.(*GoBinary)

	var d D
	d.DepTo = make(map[string][]string)
	d.GoFiles = make(map[string][]string)

	for _, platform := range platforms {
		env := platformToEnv(platform, 1)

		cfg := &packages.Config{
			Context: ctx,
			Mode:    packages.NeedImports | packages.NeedDeps | packages.NeedFiles | packages.NeedName,
			Env:     env,
			Dir:     root,
		}

		pkgs, err := packages.Load(cfg, "./"+bin.SourcePath)
		if err != nil {
			return nil, err
		}

		packages.Visit(pkgs,
			func(p *packages.Package) bool {
				pkgPath := imports.VendorlessPath(p.PkgPath)
				return strings.HasPrefix(pkgPath, bin.GoModule+"/")
			},
			func(p *packages.Package) {
				pkgPath := imports.VendorlessPath(p.PkgPath)
				if !strings.HasPrefix(pkgPath, bin.GoModule+"/") {
					return
				}

				for imp := range p.Imports {
					d.AddEdge(pkgPath, imp)
				}
				d.GoFiles[pkgPath] = p.GoFiles
				d.GoFiles[pkgPath] = append(d.GoFiles[pkgPath], p.OtherFiles...)
			})
	}
	return &d, nil
}

type D struct {
	Deps    []string
	DepTo   map[string][]string // pkg in key is imported by packages in value
	GoFiles map[string][]string
}

func (d *D) AddEdge(from, to string) {
	d.DepTo[from] = append(d.DepTo[from], imports.VendorlessPath(to))
}

func (d *D) AddDep(pkg, goos string) {
	pkg = imports.VendorlessPath(pkg)
	if isBoringPackage(pkg) {
		return
	}
	if !slices.Contains(d.Deps, pkg) {
		d.Deps = append(d.Deps, pkg)
	}
}

func isBoringPackage(pkg string) bool {
	return strings.HasPrefix(pkg, "internal/") ||
		strings.HasPrefix(pkg, "runtime/internal/") ||
		pkg == "runtime" || pkg == "runtime/cgo" || pkg == "unsafe" ||
		(strings.Contains(pkg, "/internal/") && isGoPackage(pkg))
}

func isGoPackage(pkg string) bool {
	return !strings.Contains(pkg, ".") ||
		strings.Contains(pkg, "golang.org/x")
}
