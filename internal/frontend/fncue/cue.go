// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Implements loading of Foundation-specific dialect of Cue which includes:
// * a Golang-like module system where modules are loaded from source transparently when needed;
// * support for @fn() attributes allowing to access runtime data from the environment.
package fncue

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/schema"
)

type KeyAndPath struct {
	Key    string
	Target cue.Path
}

func (v *CueV) LookupPath(path string) *CueV {
	return &CueV{Val: v.Val.LookupPath(cue.ParsePath(path))}
}

func (v *CueV) Exists() bool { return v.Val.Exists() }

func (v *CueV) FillPath(path cue.Path, rightSide interface{}) *CueV {
	return &CueV{Val: v.Val.FillPath(path, rightSide)}
}

type WorkspaceLoader interface {
	SnapshotDir(context.Context, schema.PackageName, memfs.SnapshotOpts) (fnfs.Location, error)
}

// Represents an unparsed Cue package.
type CuePackage struct {
	ModuleName string
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
	loc, err := loader.SnapshotDir(ctx, schema.PackageName(pkgName), memfs.SnapshotOpts{IncludeFilesGlobs: []string{"*.cue"}})
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
		Files:      files,
		Sources:    loc.FS,
	}, nil
}

// Entry point to load Cue packages from a Foundation workspace.
type EvalCtx struct {
	cache  *snapshotCache
	loader WorkspaceLoader
}

type snapshotCache struct {
	mu     sync.Mutex // Protects cuectx.
	cuectx *cue.Context
	bldctx *build.Context
	parsed map[string]*build.Instance
	built  map[string]*Partial
}

func NewEvalCtx(loader WorkspaceLoader) *EvalCtx {
	return &EvalCtx{cache: newSnapshotCache(), loader: loader}
}

func newSnapshotCache() *snapshotCache {
	return &snapshotCache{
		cuectx: cuecontext.New(),
		bldctx: build.NewContext(),
		parsed: map[string]*build.Instance{},
		built:  map[string]*Partial{},
	}
}

func (ev *EvalCtx) Eval(ctx context.Context, pkgname string) (*Partial, error) {
	// We work around Cue's limited package management. Rather than maintaining package copies under
	// a top-level cue.mod directory, we want instead a system more similar to Go's, with explicit
	// version locking, and downloads into a common shared cache.
	collectedImports := map[string]*CuePackage{}

	if err := CollectImports(ctx, ev.loader, pkgname, collectedImports); err != nil {
		return nil, err
	}

	pkg, ok := collectedImports[pkgname]
	if !ok || len(pkg.Files) == 0 {
		return nil, fnerrors.UserError(nil, "no cue package at %s?", pkgname)
	}

	// A foundation package definition has no package statement, which we refer to as the "_"
	// import here.
	return ev.cache.Eval(ctx, *pkg, pkgname+":_", collectedImports)
}

func joinErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		var msg bytes.Buffer
		fmt.Fprintln(&msg, "Encountered multiple errors:")
		for i, err := range errs {
			fmt.Fprintf(&msg, "Error (%d):\n%s\n", i, err)
		}
		return fmt.Errorf(msg.String())
	}
}

func (sc *snapshotCache) Eval(ctx context.Context, pkg CuePackage, pkgname string, collectedImports map[string]*CuePackage) (*Partial, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if _, has := sc.built[pkgname]; !has {
		info, _ := astutil.ParseImportSpec(ast.NewImport(nil, pkgname))
		p := sc.parseAndCacheInstance(ctx, pkg, info, collectedImports)
		if len(p.DepsErrors) > 0 {
			return nil, joinErrors(p.DepsErrors)
		}
		vv := sc.cuectx.BuildInstance(p)

		partial := &Partial{Ctx: sc}
		partial.Package = pkg
		partial.Val = vv

		var err error
		partial.Left, err = parseTags(&partial.CueV)
		if err != nil {
			return nil, err
		}

		for _, dep := range collectedImports {
			partial.CueImports = append(partial.CueImports, *dep)
		}
		sort.Slice(partial.CueImports, func(i, j int) bool {
			return strings.Compare(partial.CueImports[i].ModuleName, partial.CueImports[j].ModuleName) < 0
		})

		if vv.Err() != nil {
			// Even if there are errors, return the partially valid Cue value.
			// This is useful to provide language features in LSP for not fully valid files.
			return partial, vv.Err()
		}

		sc.built[pkgname] = partial
	}

	return sc.built[pkgname], nil
}

func (sc *snapshotCache) parseAndCacheInstance(ctx context.Context, pkg CuePackage, info astutil.ImportInfo, collectedImports map[string]*CuePackage) *build.Instance {
	if p := sc.parsed[info.ID]; p != nil {
		return p
	}

	p := sc.parseInstance(ctx, collectedImports, info, pkg)
	sc.parsed[info.ID] = p
	return p
}

func (sc *snapshotCache) parseInstance(ctx context.Context, collectedImports map[string]*CuePackage, info astutil.ImportInfo, pkg CuePackage) *build.Instance {
	p := sc.bldctx.NewInstance(fmt.Sprintf("%s/%s", pkg.ModuleName, pkg.RelPath), func(pos token.Pos, path string) *build.Instance {
		if IsStandardImportPath(path) {
			return nil // Builtin.
		}

		info, _ := astutil.ParseImportSpec(ast.NewImport(nil, path))
		if pkg, ok := collectedImports[info.Dir]; ok {
			return sc.parseAndCacheInstance(ctx, *pkg, info, collectedImports)
		}

		return nil
	})

	for _, f := range pkg.Files {
		contents, err := fs.ReadFile(pkg.Sources, filepath.Join(pkg.RelPath, f))
		if err != nil {
			p.Err = errors.Append(p.Err, errors.Promote(err, "ReadFile"))
			continue
		}

		// Filename recorded is "example.com/module/package/file.cue".
		importPath := path.Join(pkg.ModuleName, pkg.RelPath, f)
		parsed, err := parser.ParseFile(importPath, contents, parser.ParseComments)
		if err != nil {
			p.Err = errors.Append(p.Err, errors.Promote(err, "ParseFile"))
			continue
		}

		if pkgName := parsed.PackageName(); pkgName == "" {
			if info.PkgName != "_" {
				continue
			}
		} else if info.PkgName != pkgName {
			continue
		}

		if err := p.AddSyntax(parsed); err != nil {
			fmt.Fprintln(console.Stderr(ctx), "internal error: ", err)
		}
	}

	return p
}

func IsStandardImportPath(path string) bool {
	i := strings.Index(path, "/")
	if i < 0 {
		return true
	}
	elem := path[:i]
	// Does it look like a domain name?
	return !strings.Contains(elem, ".")
}

func WalkAttrs(parent cue.Value, visit func(v cue.Value, key, value string) error) error {
	var errs []error

	parent.Walk(nil, func(v cue.Value) {
		attrs := v.Attributes(cue.ValueAttr)
		for _, attr := range attrs {
			if attr.Name() != "fn" {
				continue
			}

			for k := 0; k < attr.NumArgs(); k++ {
				key, value := attr.Arg(k)
				if err := visit(v, key, value); err != nil {
					errs = append(errs, err)
				}
			}
		}
	})

	return multierr.New(errs...)
}

func parseTags(vv *CueV) ([]KeyAndPath, error) {
	var recorded []KeyAndPath

	if err := WalkAttrs(vv.Val, func(v cue.Value, key, value string) error {
		switch key {
		case InputKeyword:
			if !stringsContain(knownInputs, value) {
				return fnerrors.InternalError("%s is a not a supported value of @fn(%s)", value, InputKeyword)
			}

			recorded = append(recorded, KeyAndPath{Key: value, Target: v.Path()})

		case AllocKeyword:
			if !stringsContain(knownAllocs, value) {
				return fnerrors.InternalError("%s is a not a supported value of @fn(%s)", value, AllocKeyword)
			}

		default:
			return fnerrors.InternalError("%s is not a supported @fn keyword", key)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return recorded, nil
}

func stringsContain(strs []string, s string) bool {
	for _, str := range strs {
		if str == s {
			return true
		}
	}
	return false
}
