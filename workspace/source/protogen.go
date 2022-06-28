// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"fmt"
	"io/fs"
	"sync"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/sdk/buf"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func RegisterGraphHandlers() {
	ops.Register[*OpProtoGen](statefulGen{})
}

type statefulGen struct{}

var _ ops.BatchedDispatcher[*OpProtoGen] = statefulGen{}

func (statefulGen) Handle(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, msg *OpProtoGen) (*ops.HandleResult, error) {
	wenv, ok := env.(workspace.MutableWorkspaceEnvironment)
	if !ok {
		return nil, fnerrors.New("WorkspaceEnvironment required")
	}

	return nil, generateProtoSrcs(ctx, env, map[schema.Framework]*protos.FileDescriptorSetAndDeps{
		msg.Framework: msg.Protos,
	}, wenv.OutputFS())
}

func (statefulGen) StartSession(ctx context.Context, env ops.Environment) ops.Session[*OpProtoGen] {
	wenv, ok := env.(workspace.MutableWorkspaceEnvironment)
	if !ok {
		// An error will then be returned in Close().
		wenv = nil
	}

	return &multiGen{ctx: ctx, wenv: wenv, request: map[schema.Framework][]*protos.FileDescriptorSetAndDeps{}}
}

type multiGen struct {
	ctx  context.Context
	wenv workspace.MutableWorkspaceEnvironment

	mu      sync.Mutex
	request map[schema.Framework][]*protos.FileDescriptorSetAndDeps
}

func (m *multiGen) Handle(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, msg *OpProtoGen) (*ops.HandleResult, error) {
	loc, err := m.wenv.Resolve(ctx, schema.PackageName(msg.PackageName))
	if err != nil {
		return nil, err
	}

	if loc.Module.ModuleName() != m.wenv.ModuleName() {
		return nil, fnerrors.BadInputError("%s: can't perform codegen for packages in %q", m.wenv.ModuleName(), loc.Module.ModuleName())
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.request[msg.Framework] = append(m.request[msg.Framework], msg.Protos)

	return nil, nil
}

func (m *multiGen) Commit() error {
	if m.wenv == nil {
		return fnerrors.New("WorkspaceEnvironment required")
	}

	m.mu.Lock()
	request := map[schema.Framework]*protos.FileDescriptorSetAndDeps{}
	var errs []error
	for fmwk, p := range m.request {
		var err error
		request[fmwk], err = protos.Merge(p...)
		errs = append(errs, err)
	}
	m.mu.Unlock()

	if mergeErr := multierr.New(errs...); mergeErr != nil {
		return mergeErr
	}

	return generateProtoSrcs(m.ctx, m.wenv, request, m.wenv.OutputFS())
}

func generateProtoSrcs(ctx context.Context, env ops.Environment, request map[schema.Framework]*protos.FileDescriptorSetAndDeps, out fnfs.ReadWriteFS) error {
	protogen, err := buf.MakeProtoSrcs(ctx, env, request)
	if err != nil {
		return err
	}

	merged, err := compute.GetValue(ctx, protogen)
	if err != nil {
		return err
	}

	if console.DebugToConsole {
		d := console.Debug(ctx)

		fmt.Fprintln(d, "Codegen results:")
		_ = fnfs.VisitFiles(ctx, merged, func(path string, _ bytestream.ByteStream, _ fs.DirEntry) error {
			fmt.Fprintf(d, "  %s\n", path)
			return nil
		})
	}

	if err := fnfs.WriteFSToWorkspace(ctx, console.Stdout(ctx), out, merged); err != nil {
		return err
	}

	return nil
}

func GenProtosAtPaths(ctx context.Context, env ops.Environment, root *workspace.Root, fmwk schema.Framework, paths []string, out fnfs.ReadWriteFS) error {
	opts, err := workspace.MakeProtoParseOpts(ctx, workspace.NewPackageLoader(root), root.Workspace)
	if err != nil {
		return err
	}

	parsed, err := opts.Parse(root.FS(), paths)
	if err != nil {
		return err
	}

	return generateProtoSrcs(ctx, env, map[schema.Framework]*protos.FileDescriptorSetAndDeps{
		fmwk: parsed,
	}, out)
}
