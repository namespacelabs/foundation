// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package snapshot

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/eval"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func RequireServers(env cfg.Context, planner runtime.Planner, servers ...schema.PackageName) compute.Computable[*ServerSnapshot] {
	return &requiredServers{env: env, planner: planner, packages: servers}
}

type requiredServers struct {
	env      cfg.Context
	planner  runtime.Planner
	packages []schema.PackageName

	compute.LocalScoped[*ServerSnapshot]
}

type ServerSnapshot struct {
	stack  *planning.Stack
	sealed pkggraph.SealedPackageLoader
	env    cfg.Context
}

func (rs *requiredServers) Action() *tasks.ActionEvent {
	return tasks.Action("planning.require-servers")
}

func (rs *requiredServers) Inputs() *compute.In {
	return compute.Inputs().Indigestible("env", rs.env).Strs("packages", schema.Strs(rs.packages...))
}

func (rs *requiredServers) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (rs *requiredServers) Compute(ctx context.Context, _ compute.Resolved) (*ServerSnapshot, error) {
	return computeSnapshot(ctx, rs.env, rs.planner, rs.packages)
}

func computeSnapshot(ctx context.Context, env cfg.Context, planner runtime.Planner, packages []schema.PackageName) (*ServerSnapshot, error) {
	pl := parsing.NewPackageLoader(env)

	var servers []planning.Server
	for _, pkg := range packages {
		server, err := planning.RequireServerWith(ctx, env, pl, schema.PackageName(pkg))
		if err != nil {
			return nil, err
		}

		servers = append(servers, server)
	}

	stack, err := planning.ComputeStack(ctx, servers, planning.ProvisionOpts{Planner: planner, PortRange: eval.DefaultPortRange()})
	if err != nil {
		return nil, err
	}

	return &ServerSnapshot{stack: stack, sealed: pl.Seal(), env: env}, nil
}

func (snap *ServerSnapshot) Get(pkgs ...schema.PackageName) ([]planning.PlannedServer, error) {
	var servers []planning.PlannedServer
	for _, pkg := range pkgs {
		srv, ok := snap.stack.Get(pkg)
		if !ok {
			return nil, fnerrors.InternalError("%s: not present in the snapshot", pkg)
		}
		servers = append(servers, srv)
	}
	return servers, nil
}

func (snap *ServerSnapshot) Modules() pkggraph.Modules {
	return snap.sealed
}

func (snap *ServerSnapshot) Env() pkggraph.Context {
	return pkggraph.MakeSealedContext(snap.env, snap.sealed)
}

func (snap *ServerSnapshot) Equals(rhs *ServerSnapshot) bool {
	return false // XXX optimization.
}

func (snap *ServerSnapshot) Stack() *planning.Stack {
	return snap.stack
}
