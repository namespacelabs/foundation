// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/source/codegen"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type codegenWorkspace struct {
	srv provision.Server
}

func (cw codegenWorkspace) ModuleName() string { return cw.srv.Module().ModuleName() }
func (cw codegenWorkspace) Abs() string        { return cw.srv.Module().Abs() }
func (cw codegenWorkspace) VersionedFS(rel string, observeChanges bool) compute.Computable[wscontents.Versioned] {
	return &codegenThenSnapshot{srv: cw.srv, rel: rel, observeChanges: observeChanges}
}

type codegenThenSnapshot struct {
	srv            provision.Server
	rel            string
	observeChanges bool
	compute.LocalScoped[wscontents.Versioned]
}

func (cd *codegenThenSnapshot) Action() *tasks.ActionEvent {
	return tasks.Action("workspace.codegen-and-snapshot").Scope(cd.srv.PackageName())
}
func (cd *codegenThenSnapshot) Inputs() *compute.In {
	return compute.Inputs().Indigestible("srv", cd.srv).Str("rel", cd.rel).Bool("observeChanges", cd.observeChanges)
}
func (cd *codegenThenSnapshot) Compute(ctx context.Context, _ compute.Resolved) (wscontents.Versioned, error) {
	if err := codegenServer(ctx, cd.srv); err != nil {
		return nil, err
	}

	// Codegen is only run once; if codegen is required again, then it will be triggered
	// by a recomputation of the graph.

	return wscontents.MakeVersioned(ctx, cd.srv.Module().Abs(), cd.rel, cd.observeChanges, nil)
}

type codegenEnv struct {
	root     *workspace.Module
	packages workspace.Packages
	env      *schema.Environment
	fs       fnfs.ReadWriteFS
}

func (ce codegenEnv) ErrorLocation() string        { return ce.root.ErrorLocation() }
func (ce codegenEnv) DevHost() *schema.DevHost     { return ce.root.DevHost }
func (ce codegenEnv) Proto() *schema.Environment   { return ce.env }
func (ce codegenEnv) ModuleName() string           { return ce.root.ModuleName() }
func (ce codegenEnv) OutputFS() fnfs.ReadWriteFS   { return ce.fs }
func (ce codegenEnv) Workspace() *schema.Workspace { return ce.root.Workspace }

func (ce codegenEnv) Resolve(ctx context.Context, pkg schema.PackageName) (workspace.Location, error) {
	return ce.packages.Resolve(ctx, pkg)
}

func (ce codegenEnv) LoadByName(ctx context.Context, packageName schema.PackageName) (*workspace.Package, error) {
	return ce.packages.LoadByName(ctx, packageName)
}

func codegenServer(ctx context.Context, srv provision.Server) error {
	// XXX we should be able to disable codegen for pure builds.
	if srv.Module().IsExternal() {
		return nil
	}

	codegen, err := codegen.ForServerAndDeps(srv)
	if err != nil {
		return err
	}

	var r ops.Plan
	if err := r.Add(codegen...); err != nil {
		return err
	}

	waiters, err := r.ExecuteParallel(ctx, "workspace.codegen", codegenEnv{
		root:     srv.Module(),
		packages: srv.Env(),
		env:      srv.Env().Proto(),
		fs:       srv.Module().ReadWriteFS(),
	})
	if err != nil {
		return err
	}

	if len(waiters) > 0 {
		return fnerrors.InternalError("unexpected waiters, got %d", len(waiters))
	}

	return nil
}
