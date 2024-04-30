// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package mod

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/integrations"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/module"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	envRef           = "dev"
	foundationModule = "namespacelabs.dev/foundation"
)

func NewTidyCmd() *cobra.Command {
	var (
		env cfg.Context
	)

	return fncobra.Cmd(
		&cobra.Command{
			Use:   "tidy",
			Short: "Ensures that each server has the appropriate dependencies configured.",
			Args:  cobra.NoArgs,
		}).
		With(fncobra.HardcodeEnv(&env, envRef)).
		Do(func(ctx context.Context) error {
			// First of all, we work through all packages to make sure we have captured
			// their dependencies locally. If we don't do this here, package parsing below
			// will fail.

			if err := maybeUpdateWorkspace(ctx, env); err != nil {
				return err
			}

			root, err := module.FindRootWithArgs(ctx, ".", parsing.ModuleAtArgs{SkipAPIRequirements: true})
			if err != nil {
				return err
			}

			// Reload env since root was potentially updated.
			env, err := cfg.LoadContext(root, envRef)
			if err != nil {
				return err
			}

			pl := parsing.NewPackageLoader(env)

			list, err := parsing.ListSchemas(ctx, env, root)
			if err != nil {
				return err
			}

			packages := []*pkggraph.Package{}
			for _, loc := range list.Locations {
				pkg, err := pl.LoadByName(ctx, loc.AsPackageName())
				if err != nil {
					return err
				}

				packages = append(packages, pkg)
			}

			var errs []error
			for _, pkg := range packages {
				switch {
				case pkg.Server != nil:
					lang := integrations.IntegrationFor(pkg.Server.Framework)
					if err := lang.TidyServer(ctx, env, pl, pkg.Location, pkg.Server); err != nil {
						errs = append(errs, err)
					}

				case pkg.Node() != nil:
					for _, fmwk := range pkg.Node().CodegeneratedFrameworks() {
						lang := integrations.IntegrationFor(fmwk)
						if err := lang.TidyNode(ctx, env, pl, pkg); err != nil {
							errs = append(errs, err)
						}
					}
				}
			}
			for _, fmwk := range schema.Framework_value {
				lang := integrations.IntegrationFor(schema.Framework(fmwk))
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

func maybeUpdateWorkspace(ctx context.Context, env cfg.Context) error {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return err
	}

	res := &moduleResolver{
		deps: root.Workspace().Proto().Dep,
	}
	pl := parsing.NewPackageLoader(env, parsing.WithMissingModuleResolver(res))

	schemas, err := parsing.ListSchemas(ctx, env, root)
	if err != nil {
		return err
	}

	for _, loc := range schemas.Locations {
		if err := pl.Ensure(ctx, loc.AsPackageName()); err != nil {
			return err
		}
	}

	if root.ModuleName() != foundationModule {
		// Always add a dep on the foundation module.
		if _, err := res.Resolve(ctx, foundationModule); err != nil {
			return err
		}
	}

	return rewriteWorkspace(ctx, root, root.EditableWorkspace().WithReplacedDependencies(res.deps))
}

func rewriteWorkspace(ctx context.Context, root *parsing.Root, data pkggraph.WorkspaceData) error {
	// Write an updated workspace.ns.textpb before continuing.
	return pkggraph.WriteWorkspaceData(ctx, console.Stdout(ctx), root.ReadWriteFS(), data)
}

type moduleResolver struct {
	deps []*schema.Workspace_Dependency
}

func (r *moduleResolver) Resolve(ctx context.Context, pkg schema.PackageName) (*schema.Workspace_Dependency, error) {
	mod, err := parsing.ResolveModule(ctx, pkg.String())
	if err != nil {
		return nil, err
	}

	for _, dep := range r.deps {
		if dep.ModuleName == mod.ModuleName {
			return dep, nil
		}
	}

	dep, err := parsing.ModuleHead(ctx, mod)
	if err != nil {
		return nil, err
	}

	r.deps = append(r.deps, dep)
	return dep, nil
}
