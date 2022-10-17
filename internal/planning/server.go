// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning/compatibility"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// Represents a server bound to an environment.
type Server struct {
	Location pkggraph.Location
	Package  *pkggraph.Package

	Provisioning pkggraph.PreparedProvisionPlan // A provisioning plan that is attached to the server itself.
	Startup      pkggraph.PreStartup

	env   pkggraph.SealedContext // The environment this server instance is bound to.
	entry *schema.Stack_Entry    // The stack entry, i.e. all of the server's dependencies.
	deps  []*pkggraph.Package    // List of parsed deps.
}

func (t Server) Module() *pkggraph.Module              { return t.Location.Module }
func (t Server) SealedContext() pkggraph.SealedContext { return t.env }
func (t Server) PackageName() schema.PackageName       { return t.Location.PackageName }
func (t Server) StackEntry() *schema.Stack_Entry       { return t.entry }
func (t Server) Proto() *schema.Server                 { return t.entry.Server }
func (t Server) Name() string                          { return t.entry.Server.Name }
func (t Server) Framework() schema.Framework           { return t.entry.Server.Framework }
func (t Server) Deps() []*pkggraph.Package             { return t.deps }

func (t Server) PackageRef() *schema.PackageRef {
	return schema.MakePackageRef(t.PackageName(), t.Name())
}

func (t Server) GetDep(pkg schema.PackageName) *pkggraph.Package {
	for _, d := range t.deps {
		if d.PackageName() == pkg {
			return d
		}
	}
	return nil
}

func RequireServer(ctx context.Context, env cfg.Context, pkgname schema.PackageName) (Server, error) {
	return RequireServerWith(ctx, env, parsing.NewPackageLoader(env), pkgname)
}

func RequireServerWith(ctx context.Context, env cfg.Context, pl *parsing.PackageLoader, pkgname schema.PackageName) (Server, error) {
	return makeServer(ctx, pl, env.Environment(), pkgname, func() pkggraph.SealedContext {
		return pkggraph.MakeSealedContext(env, pl.Seal())
	})
}

func RequireLoadedServer(ctx context.Context, e pkggraph.SealedContext, pkgname schema.PackageName) (Server, error) {
	return makeServer(ctx, e, e.Environment(), pkgname, func() pkggraph.SealedContext {
		return e
	})
}

func makeServer(ctx context.Context, loader pkggraph.PackageLoader, env *schema.Environment, pkgname schema.PackageName, bind func() pkggraph.SealedContext) (Server, error) {
	sealed, err := parsing.Seal(ctx, loader, pkgname, &parsing.SealHelper{
		AdditionalServerDeps: func(fmwk schema.Framework) ([]schema.PackageName, error) {
			var pkgs schema.PackageList
			if handler, ok := parsing.FrameworkHandlers[fmwk]; ok {
				pkgs.AddMultiple(handler.DevelopmentPackages()...)
			}
			return pkgs.PackageNames(), nil
		},
	})
	if err != nil {
		return Server{}, err
	}

	if sealed.ParsedPackage == nil || sealed.ParsedPackage.Server == nil {
		return Server{}, fnerrors.UserError(pkgname, "not a server")
	}

	t := Server{
		Location: sealed.ParsedPackage.Location,
		env:      bind(),
	}

	t.Package = sealed.ParsedPackage
	t.entry = sealed.Proto
	t.deps = sealed.Deps

	if err := compatibility.CheckCompatible(env, t.entry.Server); err != nil {
		return Server{}, err
	}

	// XXX consolidate with other EvalProvision calls. This is only invoked here
	// for convenience but it's not quite right.
	pdata, err := t.Package.Parsed.EvalProvision(ctx, t.SealedContext(), pkggraph.ProvisionInputs{
		ServerLocation: t.Location,
	})
	if err != nil {
		return Server{}, fnerrors.Wrap(t.Location, err)
	}

	t.Startup = pdata.Startup
	t.Provisioning = pdata.PreparedProvisionPlan
	t.entry.ServerNaming = pdata.Naming

	return t, nil
}

type Servers []Server

func (stack Servers) Packages() schema.PackageList {
	var pl schema.PackageList
	for _, s := range stack {
		pl.Add(s.PackageName())
	}
	return pl
}
