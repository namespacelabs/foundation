// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func LoadPackageByName(ctx context.Context, env cfg.Context, name schema.PackageName) (*pkggraph.Package, error) {
	pl := NewPackageLoader(env)
	parsed, err := pl.LoadByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

// EarlyPackageLoader is available during package graph construction, and has
// the ability to load workspace contents as well. All of the contents that are
// loaded through WorkspaceOf are retained, and stored as part of the config
// image, so that package loading is fully reproducible.
type EarlyPackageLoader interface {
	pkggraph.PackageLoader
	WorkspaceOf(context.Context, *pkggraph.Module) (fs.FS, error)
}

type PackageType int

const (
	PackageType_None PackageType = iota
	PackageType_Extension
	PackageType_Service
	PackageType_Server
	PackageType_Binary
	PackageType_Test
)

type Frontend interface {
	ParsePackage(context.Context, pkggraph.Location) (*pkggraph.Package, error)
	GuessPackageType(context.Context, schema.PackageName) (PackageType, error)
}

var MakeFrontend func(EarlyPackageLoader, *schema.Environment) Frontend

// Parsing packages often as an exponential factor because nodes tend to depend on
// complete whole sub-trees. During a single root load, we maintain a cache of
// already loaded packages to minimize this fan-out cost.
type PackageLoader struct {
	absPath        string
	workspace      cfg.Workspace
	env            *schema.Environment
	frontend       Frontend
	rootmodule     *pkggraph.Module
	moduleResolver MissingModuleResolver
	mu             sync.RWMutex
	loaded         map[schema.PackageName]*pkggraph.Package // package name -> pkggraph.Package
	loading        map[schema.PackageName]*loadingPackage   // pkggraph.Package name -> loadingPackage
	loadedModules  map[string]*pkggraph.Module              // module name -> Module
}

type sealedPackages struct {
	modules  map[string]*pkggraph.Module              // module name -> Module
	packages map[schema.PackageName]*pkggraph.Package // package name -> pkggraph.Package
}

type resultPair struct {
	value *pkggraph.Package
	err   error
}

type loadingPackage struct {
	pl      *PackageLoader
	loc     pkggraph.Location
	mu      sync.Mutex
	waiters []chan resultPair
	waiting int // The first waiter, will also get to the package load.
	done    bool
	result  resultPair
}

type packageLoaderOpt func(*PackageLoader)

var _ pkggraph.SealedPackageLoader = sealedPackages{}

func WithMissingModuleResolver(moduleResolver MissingModuleResolver) packageLoaderOpt {
	return func(pl *PackageLoader) {
		pl.moduleResolver = moduleResolver
	}
}

func NewPackageLoader(env cfg.Context, opt ...packageLoaderOpt) *PackageLoader {
	pl := &PackageLoader{}
	pl.absPath = env.Workspace().LoadedFrom().AbsPath
	pl.workspace = env.Workspace()
	pl.env = env.Environment()
	pl.loaded = map[schema.PackageName]*pkggraph.Package{}
	pl.loading = map[schema.PackageName]*loadingPackage{}
	pl.loadedModules = map[string]*pkggraph.Module{}
	pl.frontend = MakeFrontend(pl, env.Environment())
	pl.rootmodule = pl.inject(env.Workspace().LoadedFrom(), env.Workspace().Proto(), "" /* version */)
	pl.moduleResolver = &defaultMissingModuleResolver{workspace: env.Workspace()}

	for _, o := range opt {
		o(pl)
	}

	return pl
}

func (pl *PackageLoader) Seal() pkggraph.SealedPackageLoader {
	sealed := sealedPackages{
		modules:  map[string]*pkggraph.Module{},
		packages: map[schema.PackageName]*pkggraph.Package{},
	}

	pl.mu.Lock()

	for name, module := range pl.loadedModules {
		sealed.modules[name] = module
	}

	for name, p := range pl.loaded {
		sealed.packages[name] = p
	}

	pl.mu.Unlock()

	return sealed
}

func (pl *PackageLoader) Resolve(ctx context.Context, original schema.PackageName) (pkggraph.Location, error) {
	packageName := maybeRewritePackage(original)

	pkg := string(packageName)

	if pkg == "" || pkg == "." {
		return pkggraph.Location{}, fnerrors.InternalError("bad package reference %q", pkg)
	}

	if packageName.Equals(pl.workspace.ModuleName()) {
		return pl.rootmodule.MakeLocation("."), nil
	} else if rel := strings.TrimPrefix(pkg, pl.workspace.ModuleName()+"/"); rel != pkg {
		return pl.rootmodule.MakeLocation(rel), nil
	}

	replaced, err := pl.MatchModuleReplace(ctx, packageName)
	if err != nil {
		return pkggraph.Location{}, err
	}

	if replaced != nil {
		return *replaced, nil
	}

	mods := pl.workspace.Proto().Dep

	// XXX longest prefix match?
	for _, mod := range mods {
		if rel := strings.TrimPrefix(pkg, mod.ModuleName+"/"); rel != pkg || pkg == mod.ModuleName {
			return pl.ExternalLocation(ctx, mod, packageName)
		}
	}

	// Resolve missing workspace dependency.
	mod, err := pl.moduleResolver.Resolve(ctx, packageName)
	if err != nil {
		return pkggraph.Location{}, err
	}

	return pl.ExternalLocation(ctx, mod, packageName)
}

func (pl *PackageLoader) MatchModuleReplace(ctx context.Context, packageName schema.PackageName) (*pkggraph.Location, error) {
	for _, replace := range pl.workspace.Proto().Replace {
		rel, ok := schema.IsParent(replace.ModuleName, packageName)
		if ok {
			module, err := pl.resolveExternal(ctx, replace.ModuleName, func() (*LocalModule, error) {
				return &LocalModule{
					ModuleName: replace.ModuleName,
					LocalPath:  filepath.Join(pl.absPath, replace.Path),
				}, nil
			})
			if err != nil {
				return nil, err
			}

			loc := module.MakeLocation(rel)
			return &loc, nil
		}
	}

	return nil, nil
}

func (pl *PackageLoader) WorkspaceOf(ctx context.Context, module *pkggraph.Module) (fs.FS, error) {
	return module.ReadOnlyFS(), nil
}

func (pl *PackageLoader) LoadByName(ctx context.Context, packageName schema.PackageName) (*pkggraph.Package, error) {
	loc, err := pl.Resolve(ctx, packageName)
	if err != nil {
		return nil, err
	}

	return pl.loadPackage(ctx, loc)
}

func (pl *PackageLoader) loadPackage(ctx context.Context, loc pkggraph.Location) (*pkggraph.Package, error) {
	pkgName := loc.PackageName

	// Fast path: was the package already loaded?
	pl.mu.RLock()
	loaded := pl.loaded[pkgName]
	pl.mu.RUnlock()
	if loaded != nil {
		return loaded, nil
	}

	// Slow path: if not, concentrate all concurrent loads of the same package into a single loader.
	pl.mu.Lock()
	loading := pl.loading[pkgName]
	if loading == nil {
		loading = &loadingPackage{
			pl:  pl,
			loc: loc,
		}
		pl.loading[pkgName] = loading
	}
	pl.mu.Unlock()

	if err := loading.Ensure(ctx); err != nil {
		return nil, err
	}

	return loading.Get(ctx)
}

func (pl *PackageLoader) ExternalLocation(ctx context.Context, mod *schema.Workspace_Dependency, packageName schema.PackageName) (pkggraph.Location, error) {
	module, err := pl.resolveExternal(ctx, mod.ModuleName, func() (*LocalModule, error) {
		return DownloadModule(ctx, mod, false)
	})
	if err != nil {
		return pkggraph.Location{}, err
	}

	if string(packageName) == module.ModuleName() {
		return module.MakeLocation("."), nil
	}

	rel := strings.TrimPrefix(string(packageName), module.ModuleName()+"/")
	if packageName.Equals(rel) {
		return pkggraph.Location{}, fnerrors.InternalError("%s: inconsistent module, got %q", packageName, module.ModuleName())
	}

	return module.MakeLocation(rel), nil
}

func (pl *PackageLoader) inject(lf *schema.Workspace_LoadedFrom, w *schema.Workspace, version string) *pkggraph.Module {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	m := pkggraph.NewModule(w, lf, version)
	pl.loadedModules[m.ModuleName()] = m
	return m
}

func (pl *PackageLoader) resolveExternal(ctx context.Context, moduleName string, download func() (*LocalModule, error)) (*pkggraph.Module, error) {
	pl.mu.RLock()
	m := pl.loadedModules[moduleName]
	pl.mu.RUnlock()
	if m != nil {
		return m, nil
	}

	downloaded, err := download()
	if err != nil {
		return nil, err
	}

	data, err := ModuleAt(ctx, downloaded.LocalPath, ModuleAtArgs{})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fnerrors.UserError(nil, "%s: is not a workspace, %q missing.", moduleName, data.DefinitionFile())
		}
		return nil, err
	}

	if data.ModuleName() != moduleName {
		return nil, fnerrors.InternalError("%s: inconsistent definition, module specified %q", moduleName, data.ModuleName())
	}

	return pl.inject(data.LoadedFrom(), data.Proto(), downloaded.Version), nil
}

type PackageLoaderStats struct {
	LoadedPackageCount int
	LoadedModuleCount  int
	PerModule          map[string][]string
}

func (pl *PackageLoader) Stats(ctx context.Context) PackageLoaderStats {
	var stats PackageLoaderStats

	pl.mu.Lock()
	defer pl.mu.Unlock()

	stats.LoadedPackageCount = len(pl.loaded)
	stats.PerModule = map[string][]string{}
	stats.LoadedModuleCount = len(pl.loadedModules)

	return stats
}

func (pl *PackageLoader) complete(pkg *pkggraph.Package) {
	pl.mu.Lock()
	pl.loaded[pkg.PackageName()] = pkg
	pl.mu.Unlock()
}

func (l *loadingPackage) Ensure(ctx context.Context) error {
	l.mu.Lock()

	rev := l.waiting
	l.waiting++

	if rev > 0 {
		// Someone is already loading the package.
		l.mu.Unlock()
		return nil
	}

	l.mu.Unlock()
	var res resultPair
	res.value, res.err = tasks.Return(ctx, tasks.Action("package.load").Scope(l.loc.PackageName), func(ctx context.Context) (*pkggraph.Package, error) {
		pp, err := l.pl.frontend.ParsePackage(ctx, l.loc)
		if err != nil {
			return nil, err
		}

		return FinalizePackage(ctx, l.pl.env, l.pl, pp)
	})

	l.mu.Lock()

	l.done = true
	l.result = res

	if res.err == nil {
		l.pl.complete(res.value)
	}

	waiters := l.waiters
	l.waiters = nil
	l.mu.Unlock()

	for _, ch := range waiters {
		ch <- res
		close(ch)
	}
	return nil
}

func (l *loadingPackage) Get(ctx context.Context) (*pkggraph.Package, error) {
	l.mu.Lock()
	if l.done {
		defer l.mu.Unlock()
		return l.result.value, l.result.err
	}

	// Very important that this is a buffered channel, else the write above will
	// block forever and deadlock package loading.
	ch := make(chan resultPair, 1)
	l.waiters = append(l.waiters, ch)
	l.mu.Unlock()

	select {
	case v, ok := <-ch:
		if !ok {
			return nil, fnerrors.InternalError("unexpected eof")
		}
		return v.value, v.err

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (sealed sealedPackages) Resolve(ctx context.Context, original schema.PackageName) (pkggraph.Location, error) {
	packageName := maybeRewritePackage(original)

	if pkg, ok := sealed.packages[packageName]; ok {
		return pkg.Location, nil
	}

	if mod, ok := sealed.modules[packageName.String()]; ok {
		return mod.MakeLocation("."), nil
	}

	return pkggraph.Location{}, fnerrors.InternalError("%s: package not loaded while resolving!", packageName)
}

func (sealed sealedPackages) LoadByName(ctx context.Context, original schema.PackageName) (*pkggraph.Package, error) {
	packageName := maybeRewritePackage(original)

	if pkg, ok := sealed.packages[packageName]; ok {
		return pkg, nil
	}

	return nil, fnerrors.InternalError("%s: package not loaded!", packageName)
}

func (sealed sealedPackages) Modules() []*pkggraph.Module {
	mods := maps.Values(sealed.modules)
	slices.SortFunc(mods, func(a, b *pkggraph.Module) bool {
		return strings.Compare(a.ModuleName(), b.ModuleName()) < 0
	})
	return mods
}

func (sealed sealedPackages) Packages() []*pkggraph.Package {
	packages := maps.Values(sealed.packages)
	slices.SortFunc(packages, func(a, b *pkggraph.Package) bool {
		return strings.Compare(a.PackageName().String(), b.PackageName().String()) < 0
	})
	return packages
}

func Ensure(ctx context.Context, packages pkggraph.PackageLoader, packageName schema.PackageName) error {
	_, err := packages.LoadByName(ctx, packageName)
	return err
}

func maybeRewritePackage(pkg schema.PackageName) schema.PackageName {
	for key, target := range schema.StaticModuleRewrites {
		if string(pkg) == key {
			return schema.PackageName(filepath.Join(target.ModuleName, target.RelPath))
		} else if strings.HasPrefix(string(pkg), key+"/") {
			rel := strings.TrimPrefix(string(pkg), key+"/")
			return schema.PackageName(filepath.Join(target.ModuleName, target.RelPath, rel))
		}
	}

	return pkg
}
