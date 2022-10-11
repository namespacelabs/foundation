// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/provision/parsed"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/source/codegen"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type codegenWorkspace struct {
	srv parsed.Server
}

func (cw codegenWorkspace) ModuleName() string { return cw.srv.Module().ModuleName() }
func (cw codegenWorkspace) Abs() string        { return cw.srv.Module().Abs() }
func (cw codegenWorkspace) VersionedFS(rel string, observeChanges bool) compute.Computable[wscontents.Versioned] {
	return &codegenThenSnapshot{srv: cw.srv, rel: rel, observeChanges: observeChanges}
}

type codegenThenSnapshot struct {
	srv            parsed.Server
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
	config   planning.Configuration
	root     *pkggraph.Module
	packages pkggraph.PackageLoader
	env      *schema.Environment
	fs       fnfs.ReadWriteFS
}

var _ pkggraph.ContextWithMutableModule = codegenEnv{}

func (ce codegenEnv) ErrorLocation() string                 { return ce.root.ErrorLocation() }
func (ce codegenEnv) Environment() *schema.Environment      { return ce.env }
func (ce codegenEnv) ModuleName() string                    { return ce.root.ModuleName() }
func (ce codegenEnv) ReadWriteFS() fnfs.ReadWriteFS         { return ce.fs }
func (ce codegenEnv) Configuration() planning.Configuration { return ce.config }
func (ce codegenEnv) Workspace() planning.Workspace         { return ce.root.WorkspaceData }

func (ce codegenEnv) Resolve(ctx context.Context, pkg schema.PackageName) (pkggraph.Location, error) {
	return ce.packages.Resolve(ctx, pkg)
}

func (ce codegenEnv) LoadByName(ctx context.Context, packageName schema.PackageName) (*pkggraph.Package, error) {
	return ce.packages.LoadByName(ctx, packageName)
}

func codegenServer(ctx context.Context, srv parsed.Server) error {
	// XXX we should be able to disable codegen for pure builds.
	if srv.Module().IsExternal() {
		return nil
	}

	codegen, err := codegen.ForServerAndDeps(srv)
	if err != nil {
		return err
	}

	if len(codegen) == 0 {
		return nil
	}

	r := execution.NewPlan(codegen...)

	return execution.Execute(ctx, srv.SealedContext(), "workspace.codegen", r, nil,
		pkggraph.MutableModuleInjection.With(srv.Module()),
		pkggraph.PackageLoaderInjection.With(srv.SealedContext()),
	)
}
