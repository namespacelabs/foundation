// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncue

import (
	"context"
	"io/fs"
	"path/filepath"

	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/parser"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/schema"
)

// Represents an unparsed Cue package.
type CuePackage struct {
	ModuleName string
	AbsPath    string
	RelPath    string   // Relative to module root.
	Files      []string // Relative to RelPath
	Sources    fs.FS
	Imports    []string // Top level import statements.
}

func (pkg CuePackage) RelFiles() []string {
	var files []string
	for _, f := range pkg.Files {
		files = append(files, filepath.Join(pkg.RelPath, f))
	}
	return files
}

type WorkspaceLoader interface {
	SnapshotDir(context.Context, schema.PackageName, memfs.SnapshotOpts) (loc fnfs.Location, absPath string, err error)
}

// Fills [m] with the transitive closure of packages and files imported by package [pkgname].
// TODO: Use [snapshotCache] instead of re-parsing all packages directly.
func CollectImports(ctx context.Context, resolver WorkspaceLoader, pkgname string, m map[string]*CuePackage) error {
	if _, ok := m[pkgname]; ok {
		return nil
	}

	// Leave a marker that this package is already handled, to avoid processing through cycles.
	m[pkgname] = &CuePackage{}

	pkg, err := loadPackageContents(ctx, resolver, pkgname)
	if err != nil {
		return err
	}

	m[pkgname] = pkg

	if len(pkg.Files) == 0 {
		return nil
	}

	for _, fp := range pkg.RelFiles() {
		contents, err := fs.ReadFile(pkg.Sources, fp)
		if err != nil {
			return err
		}

		f, err := parser.ParseFile(fp, contents, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, imp := range f.Imports {
			importInfo, _ := astutil.ParseImportSpec(imp)
			pkg.Imports = append(pkg.Imports, importInfo.Dir)
			if IsStandardImportPath(importInfo.ID) {
				continue
			}

			if err := CollectImports(ctx, resolver, importInfo.Dir, m); err != nil {
				return err
			}
		}
	}

	return nil
}

func loadPackageContents(ctx context.Context, loader WorkspaceLoader, pkgName string) (*CuePackage, error) {
	loc, absPath, err := loader.SnapshotDir(ctx, schema.PackageName(pkgName), memfs.SnapshotOpts{IncludeFilesGlobs: []string{"*.cue"}})
	if err != nil {
		return nil, err
	}

	fifs, err := fs.ReadDir(loc.FS, loc.RelPath)
	if err != nil {
		return nil, err
	}

	// We go wide here and don't take packages into account. Packages are then filtered while building.
	var files []string
	for _, f := range fifs {
		if f.IsDir() || filepath.Ext(f.Name()) != ".cue" {
			continue
		}

		files = append(files, f.Name())
	}

	return &CuePackage{
		ModuleName: loc.ModuleName,
		RelPath:    loc.RelPath,
		AbsPath:    absPath,
		Files:      files,
		Sources:    loc.FS,
	}, nil
}
