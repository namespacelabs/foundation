// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Implements loading of Namespace-specific dialect of Cue which includes:
// * a Golang-like module system where modules are loaded from source transparently when needed;
// * support for @fn() attributes allowing to access runtime data from the environment.
package fncue

import (
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
	SnapshotDir(context.Context, schema.PackageName, memfs.SnapshotOpts) (loc fnfs.Location, absPath string, err error)
}

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

// Entry point to load Cue packages from a Namespace workspace.
type EvalCtx struct {
	cache  *snapshotCache
	loader WorkspaceLoader
	scope  interface{}
}

type snapshotCache struct {
	mu     sync.Mutex // Protects cuectx.
	cuectx *cue.Context
	bldctx *build.Context
	parsed map[string]*build.Instance
	built  map[string]*Partial
}

// If set, "scope" are passed as a "Scope" BuildOption to "BuildInstance".
func NewEvalCtx(loader WorkspaceLoader, scope interface{}) *EvalCtx {
	return &EvalCtx{
		cache:  newSnapshotCache(),
		loader: loader,
		scope:  scope}
}

func newSnapshotCache() *snapshotCache {
	return &snapshotCache{
		cuectx: cuecontext.New(),
		bldctx: build.NewContext(),
		parsed: map[string]*build.Instance{},
		built:  map[string]*Partial{},
	}
}

func (ev *EvalCtx) EvalPackage(ctx context.Context, pkgname string) (*Partial, error) {
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
	return ev.cache.Eval(ctx, *pkg, pkgname+":_", collectedImports, ev.scope)
}

func EvalWorkspace(ctx context.Context, fsys fs.FS, dir string, files []string) (*Partial, error) {
	bldctx := build.NewContext()

	p := bldctx.NewInstance(dir, func(pos token.Pos, path string) *build.Instance {
		if IsStandardImportPath(path) {
			return nil // Builtin.
		}

		berr := bldctx.NewInstance(dir, nil)
		berr.Err = errors.Promote(fnerrors.New("imports not allowed"), "")
		return berr
	})

	pkg := CuePackage{
		RelPath: ".",
		Files:   files,
		Sources: fsys,
	}

	if err := parseSources(ctx, p, "_", pkg); err != nil {
		return nil, err
	}

	return finishInstance(nil, cuecontext.New(), p, pkg, nil, nil)
}

func (sc *snapshotCache) Eval(ctx context.Context, pkg CuePackage, pkgname string, collectedImports map[string]*CuePackage, scope interface{}) (*Partial, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if _, has := sc.built[pkgname]; !has {
		info, _ := astutil.ParseImportSpec(ast.NewImport(nil, pkgname))
		p := sc.parseAndCacheInstance(ctx, pkg, info, collectedImports)
		if len(p.DepsErrors) > 0 {
			return nil, multierr.New(p.DepsErrors...)
		}

		partial, err := finishInstance(sc, sc.cuectx, p, pkg, collectedImports, scope)
		if err != nil {
			return partial, err
		}

		sc.built[pkgname] = partial
	}

	return sc.built[pkgname], nil
}

func finishInstance(sc *snapshotCache, cuectx *cue.Context, p *build.Instance, pkg CuePackage, collectedImports map[string]*CuePackage, scope interface{}) (*Partial, error) {
	buildOptions := []cue.BuildOption{}

	if scope != nil {
		buildOptions = append(buildOptions, cue.Scope(cuectx.Encode(scope)))
	}

	vv := cuectx.BuildInstance(p, buildOptions...)

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
		return partial, WrapCueError(vv.Err(), func(p string) string {
			// VSCode only supports linking of absolute paths in Output Channels.
			// Also in the Terminal it surely does not support module paths (it will link
			// example.com/module/package/path, but won't find example.com in the workspace).
			// So currently we must print absolute paths here.
			// Alternatives: print relative paths for workspace files and install a
			// DocumentLinkProvider to resolve them.
			// See https://github.com/microsoft/vscode/issues/586.
			return absPathForModulePath(collectedImports, p)
		})
	}

	return partial, nil
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
	p := sc.bldctx.NewInstance(join(pkg.ModuleName, pkg.RelPath), func(pos token.Pos, path string) *build.Instance {
		if IsStandardImportPath(path) {
			return nil // Builtin.
		}

		info, _ := astutil.ParseImportSpec(ast.NewImport(nil, path))
		if pkg, ok := collectedImports[info.Dir]; ok {
			return sc.parseAndCacheInstance(ctx, *pkg, info, collectedImports)
		}

		return nil
	})

	if err := parseSources(ctx, p, info.PkgName, pkg); err != nil {
		fmt.Fprintln(console.Errors(ctx), "internal error: ", err)
	}

	return p
}

func parseSources(ctx context.Context, p *build.Instance, expectedPkg string, pkg CuePackage) error {
	for _, f := range pkg.Files {
		contents, err := fs.ReadFile(pkg.Sources, filepath.Join(pkg.RelPath, f))
		if err != nil {
			p.Err = errors.Append(p.Err, errors.Promote(err, "ReadFile"))
			continue
		}

		// Filename recorded is "example.com/module/package/file.cue".
		importPath := filepath.Join(pkg.ModuleName, pkg.RelPath, f)
		parsed, err := parser.ParseFile(importPath, contents, parser.ParseComments)
		if err != nil {
			p.Err = errors.Append(p.Err, errors.Promote(err, "ParseFile"))
			continue
		}

		if pkgName := parsed.PackageName(); pkgName == "" {
			if expectedPkg != "_" {
				continue
			}
		} else if expectedPkg != pkgName {
			continue
		}

		if err := p.AddSyntax(parsed); err != nil {
			return err
		}
	}

	return nil
}

func join(dir, base string) string {
	if base == "." {
		return dir
	}
	return fmt.Sprintf("%s/%s", dir, base)
}

func absPathForModulePath(collectedImports map[string]*CuePackage, p string) string {
	for _, pkg := range collectedImports {
		pkgRoot := path.Join(pkg.ModuleName, pkg.RelPath) + "/"
		if relPath := strings.TrimPrefix(p, pkgRoot); relPath != p {
			return path.Join(pkg.AbsPath, relPath)
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
