// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Packages interface {
	Resolve(ctx context.Context, packageName schema.PackageName) (Location, error)
	LoadByName(ctx context.Context, packageName schema.PackageName) (*Package, error)
	Ensure(ctx context.Context, packageName schema.PackageName) error
}

type ModuleSources struct {
	Module   *Module
	Snapshot fs.FS
}

type SealedPackages interface {
	Packages
	Sources() []ModuleSources
}

func LoadPackageByName(ctx context.Context, root *Root, name schema.PackageName, opts ...LoadPackageOpt) (*Package, error) {
	pl := NewPackageLoader(root)
	parsed, err := pl.LoadByNameWithOpts(ctx, name, opts...)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

type LoadPackageOpt func(*LoadPackageOpts)

func DontLoadDependencies() LoadPackageOpt {
	return func(lpo *LoadPackageOpts) {
		lpo.LoadPackageReferences = false
	}
}

type LoadPackageOpts struct {
	LoadPackageReferences bool
}

type EarlyPackageLoader interface {
	Packages
	WorkspaceOf(context.Context, *Module) (*memfs.IncrementalFS, error)
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
	ParsePackage(context.Context, Location, LoadPackageOpts) (*Package, error)
	GuessPackageType(context.Context, schema.PackageName) (PackageType, error)
}

var MakeFrontend func(EarlyPackageLoader) Frontend

// Parsing packages often as an exponential factor because nodes tend to depend on
// complete whole sub-trees. During a single root load, we maintain a cache of
// already loaded packages to minimize this fan-out cost.
type PackageLoader struct {
	absPath       string
	workspace     *schema.Workspace
	workspaceData WorkspaceData
	devHost       *schema.DevHost
	frontend      Frontend
	rootmodule    *Module
	mu            sync.RWMutex
	fsys          map[string]*memfs.IncrementalFS        // module name -> IncrementalFS
	loaded        map[schema.PackageName]*Package        // package name -> Package
	loading       map[schema.PackageName]*loadingPackage // Package name -> loadingPackage
	loadedModules map[string]*Module                     // module name -> Module
}

type sealedPackages struct {
	sources  []ModuleSources
	modules  map[string]*Module              // module name -> Module
	packages map[schema.PackageName]*Package // package name -> Package
}

type resultPair struct {
	value *Package
	err   error
}

type loadingPackage struct {
	pl      *PackageLoader
	loc     Location
	opts    LoadPackageOpts
	mu      sync.Mutex
	waiters []chan resultPair
	waiting int // The first waiter, will also get to the package load.
	done    bool
	result  resultPair
}

func NewPackageLoader(root *Root) *PackageLoader {
	pl := &PackageLoader{}
	pl.absPath = root.absPath
	pl.workspace = root.Workspace
	pl.workspaceData = root.WorkspaceData
	pl.devHost = root.DevHost
	pl.loaded = map[schema.PackageName]*Package{}
	pl.loading = map[schema.PackageName]*loadingPackage{}
	pl.fsys = map[string]*memfs.IncrementalFS{}
	pl.loadedModules = map[string]*Module{}
	pl.frontend = MakeFrontend(pl)
	pl.rootmodule = pl.inject(root.absPath, root.WorkspaceData, "" /* version */)
	return pl
}

func (pl *PackageLoader) Seal() SealedPackages {
	sealed := sealedPackages{
		modules:  map[string]*Module{},
		packages: map[schema.PackageName]*Package{},
	}

	pl.mu.Lock()

	for name, module := range pl.loadedModules {
		sealed.modules[name] = module
	}

	for name, fs := range pl.fsys {
		sealed.sources = append(sealed.sources, ModuleSources{
			Module:   sealed.modules[name],
			Snapshot: fs.Clone(),
		})
	}

	for name, p := range pl.loaded {
		sealed.packages[name] = p
	}

	pl.mu.Unlock()

	sort.Slice(sealed.sources, func(i, j int) bool {
		return strings.Compare(sealed.sources[i].Module.ModuleName(), sealed.sources[j].Module.ModuleName()) < 0
	})

	return sealed
}

func (pl *PackageLoader) Resolve(ctx context.Context, packageName schema.PackageName) (Location, error) {
	pkg := string(packageName)

	if packageName.Equals(pl.workspace.ModuleName) {
		return Location{
			Module:      pl.rootmodule,
			PackageName: packageName,
			relPath:     ".",
		}, nil
	} else if rel := strings.TrimPrefix(pkg, pl.workspace.ModuleName+"/"); rel != pkg {
		return Location{
			Module:      pl.rootmodule,
			PackageName: packageName,
			relPath:     rel,
		}, nil
	}

	replaced, err := pl.MatchModuleReplace(ctx, packageName)
	if err != nil {
		return Location{}, err
	}

	if replaced != nil {
		return *replaced, nil
	}

	mods := pl.workspace.Dep

	// XXX longest prefix match?
	for _, mod := range mods {
		if rel := strings.TrimPrefix(pkg, mod.ModuleName+"/"); rel != pkg || pkg == mod.ModuleName {
			return pl.ExternalLocation(ctx, mod, packageName)
		}
	}

	return Location{}, fnerrors.UsageError("Run `ns tidy`.", "%s: missing entry in %s: run:\n  ns tidy", packageName, pl.workspaceData.DefinitionFile())
}

func (pl *PackageLoader) MatchModuleReplace(ctx context.Context, packageName schema.PackageName) (*Location, error) {
	for _, replace := range pl.workspace.Replace {
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

			return &Location{
				Module:      module,
				PackageName: packageName,
				relPath:     rel,
			}, nil
		}
	}

	return nil, nil
}

func (pl *PackageLoader) WorkspaceOf(ctx context.Context, module *Module) (*memfs.IncrementalFS, error) {
	moduleName := module.ModuleName()

	pl.mu.RLock()
	fsys := pl.fsys[moduleName]
	pl.mu.RUnlock()

	if fsys == nil {
		return nil, fnerrors.InternalError("%s: no fsys?", moduleName)
	}

	return fsys, nil
}

func (pl *PackageLoader) LoadByName(ctx context.Context, packageName schema.PackageName) (*Package, error) {
	return pl.LoadByNameWithOpts(ctx, packageName)
}

func (pl *PackageLoader) LoadByNameWithOpts(ctx context.Context, packageName schema.PackageName, opt ...LoadPackageOpt) (*Package, error) {
	loc, err := pl.Resolve(ctx, packageName)
	if err != nil {
		return nil, err
	}

	return pl.loadPackage(ctx, loc, opt...)
}

func (pl *PackageLoader) Ensure(ctx context.Context, packageName schema.PackageName) error {
	loc, err := pl.Resolve(ctx, packageName)
	if err != nil {
		return err
	}

	pl.mu.Lock()
	loading := pl.loading[packageName]
	if loading != nil {
		// Someone is already loading the package
		pl.mu.Unlock()
		return nil
	}
	loading = &loadingPackage{
		pl:   pl,
		loc:  loc,
		opts: LoadPackageOpts{LoadPackageReferences: true},
	}

	pl.loading[packageName] = loading
	pl.mu.Unlock()

	return loading.Ensure(ctx)
}

func (pl *PackageLoader) loadPackage(ctx context.Context, loc Location, opt ...LoadPackageOpt) (*Package, error) {
	opts := LoadPackageOpts{LoadPackageReferences: true}
	for _, o := range opt {
		o(&opts)
	}

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
			pl:   pl,
			loc:  loc,
			opts: opts,
		}
		pl.loading[pkgName] = loading
	}
	pl.mu.Unlock()

	if err := loading.Ensure(ctx); err != nil {
		return nil, err
	}

	return loading.Get(ctx)
}

func (pl *PackageLoader) ExternalLocation(ctx context.Context, mod *schema.Workspace_Dependency, packageName schema.PackageName) (Location, error) {
	module, err := pl.resolveExternal(ctx, mod.ModuleName, func() (*LocalModule, error) {
		return DownloadModule(ctx, mod, false)
	})
	if err != nil {
		return Location{}, err
	}

	if string(packageName) == module.ModuleName() {
		return Location{
			Module:      module,
			PackageName: packageName,
			relPath:     ".",
		}, nil
	}

	rel := strings.TrimPrefix(string(packageName), module.ModuleName()+"/")
	if packageName.Equals(rel) {
		return Location{}, fnerrors.InternalError("%s: inconsistent module, got %q", packageName, module.ModuleName())
	}

	return Location{
		Module:      module,
		PackageName: packageName,
		relPath:     rel,
	}, nil
}

func (pl *PackageLoader) inject(absPath string, w WorkspaceData, version string) *Module {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	m := &Module{
		Workspace:     w.Parsed(),
		WorkspaceData: w,
		DevHost:       pl.devHost,

		absPath: absPath,
		version: version,
	}

	pl.loadedModules[m.ModuleName()] = m
	pl.fsys[m.ModuleName()] = memfs.IncrementalSnapshot(fnfs.Local(w.AbsPath()))

	pl.fsys[m.ModuleName()].Direct().Add(w.DefinitionFile(), w.RawData())

	return m
}

func (pl *PackageLoader) resolveExternal(ctx context.Context, moduleName string, download func() (*LocalModule, error)) (*Module, error) {
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

	if data.Parsed().ModuleName != moduleName {
		return nil, fnerrors.InternalError("%s: inconsistent definition, module specified %q", moduleName, data.Parsed().ModuleName)
	}

	return pl.inject(downloaded.LocalPath, data, downloaded.Version), nil
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
	for name, mod := range pl.fsys {
		name := name // Close mod.

		// Ignore errors; we're best effort.
		_ = fnfs.VisitFiles(ctx, mod, func(path string, _ bytestream.ByteStream, _ fs.DirEntry) error {
			stats.PerModule[name] = append(stats.PerModule[name], path)
			return nil
		})
	}

	stats.LoadedModuleCount = len(pl.loadedModules)

	return stats
}

func (pl *PackageLoader) complete(pkg *Package) {
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
	res.value, res.err = tasks.Return(ctx, tasks.Action("package.load").Scope(l.loc.PackageName), func(ctx context.Context) (*Package, error) {
		return l.pl.frontend.ParsePackage(ctx, l.loc, l.opts)
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

func (l *loadingPackage) Get(ctx context.Context) (*Package, error) {
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

func (sealed sealedPackages) Resolve(ctx context.Context, packageName schema.PackageName) (Location, error) {
	if pkg, ok := sealed.packages[packageName]; ok {
		return pkg.Location, nil
	}

	if mod, ok := sealed.modules[packageName.String()]; ok {
		return Location{Module: mod, PackageName: packageName, relPath: "."}, nil
	}

	return Location{}, fnerrors.InternalError("%s: package not loaded while resolving!", packageName)
}

func (sealed sealedPackages) Ensure(ctx context.Context, packageName schema.PackageName) error {
	_, err := sealed.LoadByName(ctx, packageName)
	return err
}

func (sealed sealedPackages) LoadByName(ctx context.Context, packageName schema.PackageName) (*Package, error) {
	if pkg, ok := sealed.packages[packageName]; ok {
		return pkg, nil
	}

	return nil, fnerrors.InternalError("%s: package not loaded! See https://docs.namespace.so/reference/debug#package-loading", packageName)
}

func (sealed sealedPackages) Sources() []ModuleSources {
	return sealed.sources
}
