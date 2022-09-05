// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"io"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/module"
)

func NewTidyCmd() *cobra.Command {
	var (
		env planning.Context
	)

	return fncobra.Cmd(
		&cobra.Command{
			Use:   "tidy",
			Short: "Ensures that each server has the appropriate dependencies configured.",
			Args:  cobra.NoArgs,
		}).
		With(fncobra.FixedEnv(&env, "dev")).
		DoWithArgs(func(ctx context.Context, args []string) error {
			// First of all, we work through all packages to make sure we have captured
			// their dependencies locally. If we don't do this here, package parsing below
			// will fail.

			if err := maybeUpdateWorkspace(ctx, env); err != nil {
				return err
			}

			root, err := module.FindRootWithArgs(ctx, ".", workspace.ModuleAtArgs{SkipAPIRequirements: true})
			if err != nil {
				return err
			}

			pl := workspace.NewPackageLoader(env)

			list, err := workspace.ListSchemas(ctx, env, root)
			if err != nil {
				return err
			}

			packages := []*pkggraph.Package{}
			for _, loc := range list.Locations {
				pkg, err := pl.LoadByName(ctx, loc.AsPackageName())
				if err != nil {
					return err
				}

				if pkg.Binary != nil {
					continue
				}

				packages = append(packages, pkg)
			}

			var errs []error
			for _, pkg := range packages {
				switch {
				case pkg.Server != nil:
					if pkg.Server.Integration != nil {
						// TODO: support tidy for opaque servers.
						continue
					}
					lang := languages.IntegrationFor(pkg.Server.Framework)
					if err := lang.TidyServer(ctx, env, pl, pkg.Location, pkg.Server); err != nil {
						errs = append(errs, err)
					}

				case pkg.Node() != nil:
					for _, fmwk := range pkg.Node().CodegeneratedFrameworks() {
						lang := languages.IntegrationFor(fmwk)
						if err := lang.TidyNode(ctx, env, pl, pkg); err != nil {
							errs = append(errs, err)
						}
					}
				}
			}
			for _, fmwk := range schema.Framework_value {
				lang := languages.IntegrationFor(schema.Framework(fmwk))
				if lang == nil {
					continue
				}
				if err := lang.TidyWorkspace(ctx, env, packages); err != nil {
					errs = append(errs, err)
				}
			}

			return multierr.New(errs...)
		})
}

func maybeUpdateWorkspace(ctx context.Context, env planning.Context) error {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return err
	}

	pl := workspace.NewPackageLoader(env)

	if err := fillDependencies(ctx, root, pl, env); err != nil {
		return err
	}

	return nil
}

func fillDependencies(ctx context.Context, root *workspace.Root, pl *workspace.PackageLoader, env planning.Context) error {
	locs, err := listLocations(ctx, root)
	if err != nil {
		return err
	}

	alloc := &allocator{
		loader:   pl,
		root:     root,
		ws:       protos.Clone(root.Workspace()),
		resolved: map[string]*schema.Workspace_Dependency{},
		modules:  map[string]*schema.Workspace_Dependency{},
		left:     locs,
		env:      env,
	}

	for _, dep := range root.Workspace().Dep {
		alloc.modules[dep.ModuleName] = dep
	}

	for {
		alloc.mu.Lock()
		var loc *fnfs.Location
		if len(alloc.left) > 0 {
			loc = &alloc.left[0]
			alloc.left = alloc.left[1:]
		}
		alloc.mu.Unlock()

		if loc == nil {
			break
		}

		r := &workspaceLoader{alloc}
		imports := map[string]*fncue.CuePackage{}

		// Check whether imports refer to packages; we'll see calls to workspaceResolver.
		// We ignore errors, because some of the errors may be related to the lack of
		// presence of packages.
		_ = fncue.CollectImports(ctx, r, loc.AsPackageName().String(), imports)

		parsed, err := alloc.loader.LoadByNameWithOpts(ctx, loc.AsPackageName(), workspace.DontLoadDependencies())
		if err != nil {
			return err
		}

		switch {
		case parsed.Server != nil:
			if err := alloc.checkResolves(ctx, parsed.Server.Import, parsed.Server.Reference); err != nil {
				return err
			}
		case parsed.Service != nil, parsed.Extension != nil:
			if err := alloc.checkResolves(ctx, parsed.Node().Import, parsed.Node().Reference); err != nil {
				return err
			}
		}
	}

	if root.Workspace().ModuleName != foundationModule {
		// Always add a dep on the foundation module.
		if _, err := alloc.checkResolve(ctx, schema.PackageName(foundationModule)); err != nil {
			return err
		}
	}

	modules := map[string]*schema.Workspace_Dependency{}

	var deps []*schema.Workspace_Dependency
	for _, dep := range alloc.resolved {
		if modules[dep.ModuleName] != nil {
			continue
		}

		modules[dep.ModuleName] = dep
		deps = append(deps, dep)
	}

	return rewriteWorkspace(ctx, root, root.EditableWorkspace().WithReplacedDependencies(deps))
}

func rewriteWorkspace(ctx context.Context, root *workspace.Root, data pkggraph.WorkspaceData) error {
	// Write an updated workspace.ns.textpb before continuing.
	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), root.ReadWriteFS(), data.DefinitionFile(), func(w io.Writer) error {
		return data.FormatTo(w)
	})
}

const foundationModule = "namespacelabs.dev/foundation"

type allocator struct {
	loader   *workspace.PackageLoader
	root     *workspace.Root
	ws       *schema.Workspace                       // Temporary workspace that accumulates deps (for package loading)
	mu       sync.Mutex                              // Protects resolved and left.
	modules  map[string]*schema.Workspace_Dependency // Previously loaded modules (i.e. already part of the workspace definition.)
	resolved map[string]*schema.Workspace_Dependency // Newly resolved modules.
	left     []fnfs.Location
	env      planning.Context
}

func (alloc *allocator) checkResolves(ctx context.Context, pkgs []string, refs []*schema.Reference) error {
	for _, pkg := range pkgs {
		if _, err := alloc.checkResolve(ctx, schema.PackageName(pkg)); err != nil {
			return err
		}
	}

	for _, ref := range refs {
		if ref.PackageName == "" {
			continue
		}
		if _, err := alloc.checkResolve(ctx, schema.PackageName(ref.PackageName)); err != nil {
			return err
		}
	}

	return nil
}

func (alloc *allocator) checkResolve(ctx context.Context, sch schema.PackageName) (pkggraph.Location, error) {
	if _, ok := schema.IsParent(alloc.root.Workspace().ModuleName, sch); ok {
		return alloc.loader.Resolve(ctx, sch)
	}

	// Check if we already processed this package.
	alloc.mu.Lock()
	resolved := alloc.resolved[sch.String()]
	alloc.mu.Unlock()

	var didResolve bool
	if resolved == nil {
		// First, is there a replace statement that applies to this package?
		replaced, err := alloc.loader.MatchModuleReplace(ctx, sch)
		if err != nil {
			return pkggraph.Location{}, err
		}

		// If so, there's nothing for us to do here.
		if replaced != nil {
			return alloc.loader.Resolve(ctx, sch)
		}

		// Then, resolve the package to a module name + relative path.
		mod, err := workspace.ResolveModule(ctx, sch.String())
		if err != nil {
			return pkggraph.Location{}, err
		}

		// Check if we already parsed this module.
		alloc.mu.Lock()
		resolved = alloc.modules[mod.ModuleName]
		alloc.mu.Unlock()

		// No? Then fetch the latest head.
		if resolved == nil {
			dep, err := workspace.ModuleHead(ctx, mod)
			if err != nil {
				return pkggraph.Location{}, err
			}
			resolved = dep

			// Add dep and reload package loader for new deps
			alloc.ws.Dep = append(alloc.ws.Dep, dep)
			alloc.loader = workspace.NewPackageLoader(fixedWorkspace{alloc.ws, alloc.root.WorkspaceLoadedFrom(), alloc.root.DevHost(), alloc.env.Environment()})
		}

		didResolve = true
	}

	loc, err := alloc.loader.ExternalLocation(ctx, resolved, sch)
	if err == nil && didResolve {
		alloc.mu.Lock()
		alloc.resolved[sch.String()] = resolved
		alloc.modules[resolved.ModuleName] = resolved
		// If we just parsed this package, add it to the queue of packages to
		// be checked for references as well.
		alloc.left = append(alloc.left, fnfs.Location{
			ModuleName: loc.Module.ModuleName(),
			RelPath:    loc.Rel(),
		})
		alloc.mu.Unlock()
	}

	return loc, err
}

type fixedWorkspace struct {
	ws      *schema.Workspace
	lf      *schema.Workspace_LoadedFrom
	devhost *schema.DevHost
	env     *schema.Environment
}

func (fw fixedWorkspace) ErrorLocation() string                             { return fw.ws.ModuleName }
func (fw fixedWorkspace) Workspace() *schema.Workspace                      { return fw.ws }
func (fw fixedWorkspace) WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom { return fw.lf }
func (fw fixedWorkspace) DevHost() *schema.DevHost                          { return fw.devhost }
func (fw fixedWorkspace) Environment() *schema.Environment                  { return fw.env }

type workspaceLoader struct {
	alloc *allocator
}

func (wr *workspaceLoader) SnapshotDir(ctx context.Context, sch schema.PackageName, opts memfs.SnapshotOpts) (fnfs.Location, string, error) {
	loc, err := wr.alloc.checkResolve(ctx, sch)
	if err != nil {
		return fnfs.Location{}, "", err
	}

	fsys, err := memfs.SnapshotDir(fnfs.Local(loc.Module.Abs()), loc.Rel(), opts)
	if err != nil {
		return fnfs.Location{}, "", err
	}

	return fnfs.Location{
		ModuleName: loc.Module.ModuleName(),
		RelPath:    loc.Rel(),
		FS:         fsys,
	}, loc.Abs(), nil
}

func listLocations(ctx context.Context, root *workspace.Root) ([]fnfs.Location, error) {
	var locs []fnfs.Location

	visited := map[string]struct{}{} // Map of directory name to presence.

	if err := fs.WalkDir(root.ReadOnlyFS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if dirs.IsExcluded(path, d.Name()) {
				return fs.SkipDir
			}
			return nil
		}

		// Is there a least a .cue file in the directory?
		if filepath.Ext(d.Name()) == ".cue" {
			dir := filepath.Dir(path)
			if _, ok := visited[dir]; ok {
				return nil
			}

			pkg := root.RelPackage(dir)
			locs = append(locs, pkg)

			visited[dir] = struct{}{}
		}

		return nil
	}); err != nil {
		return []fnfs.Location{}, err
	}

	return locs, nil
}
