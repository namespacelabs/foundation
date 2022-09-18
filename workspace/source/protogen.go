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
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func RegisterGraphHandlers() {
	ops.Register[*OpProtoGen](statefulGen{})
}

type statefulGen struct{}

var _ ops.BatchedDispatcher[*OpProtoGen] = statefulGen{}

func (statefulGen) Handle(ctx context.Context, _ *schema.SerializedInvocation, msg *OpProtoGen) (*ops.HandleResult, error) {
	config, err := ops.Get(ctx, ops.ConfigurationInjection)
	if err != nil {
		return nil, err
	}

	module, err := ops.Get(ctx, pkggraph.MutableModuleInjection)
	if err != nil {
		return nil, err
	}

	return nil, generateProtoSrcs(ctx, config, map[schema.Framework]*protos.FileDescriptorSetAndDeps{
		msg.Framework: msg.Protos,
	}, module.ReadWriteFS())
}

func (statefulGen) PlanOrder(*OpProtoGen) (*schema.ScheduleOrder, error) {
	return nil, nil
}

func (statefulGen) StartSession(ctx context.Context) (ops.Session[*OpProtoGen], error) {
	module, err := ops.Get(ctx, pkggraph.MutableModuleInjection)
	if err != nil {
		return nil, err
	}

	loader, err := ops.Get(ctx, pkggraph.PackageLoaderInjection)
	if err != nil {
		return nil, err
	}

	config, err := ops.Get(ctx, ops.ConfigurationInjection)
	if err != nil {
		return nil, err
	}

	return &multiGen{
		ctx:     ctx,
		loader:  loader,
		module:  module,
		config:  config,
		request: map[schema.Framework][]*protos.FileDescriptorSetAndDeps{},
	}, nil
}

type multiGen struct {
	ctx    context.Context
	loader pkggraph.PackageLoader
	module pkggraph.MutableModule
	config planning.Configuration

	mu      sync.Mutex
	request map[schema.Framework][]*protos.FileDescriptorSetAndDeps
}

func (m *multiGen) Handle(ctx context.Context, _ *schema.SerializedInvocation, msg *OpProtoGen) (*ops.HandleResult, error) {
	loc, err := m.loader.Resolve(ctx, schema.PackageName(msg.PackageName))
	if err != nil {
		return nil, err
	}

	if loc.Module.ModuleName() != m.module.ModuleName() {
		return nil, fnerrors.BadInputError("%s: can't perform codegen for packages in %q", m.module.ModuleName(), loc.Module.ModuleName())
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.request[msg.Framework] = append(m.request[msg.Framework], msg.Protos)

	return nil, nil
}

func (m *multiGen) Commit() error {
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

	return generateProtoSrcs(m.ctx, m.config, request, m.module.ReadWriteFS())
}

func generateProtoSrcs(ctx context.Context, env planning.Configuration, request map[schema.Framework]*protos.FileDescriptorSetAndDeps, out fnfs.ReadWriteFS) error {
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

func GenProtosAtPaths(ctx context.Context, env planning.Context, fmwk schema.Framework, fsys fs.FS, paths []string, out fnfs.ReadWriteFS) error {
	opts, err := workspace.MakeProtoParseOpts(ctx, workspace.NewPackageLoader(env), env.Workspace().Proto())
	if err != nil {
		return err
	}

	parsed, err := opts.Parse(fsys, paths)
	if err != nil {
		return err
	}

	return generateProtoSrcs(ctx, env.Configuration(), map[schema.Framework]*protos.FileDescriptorSetAndDeps{
		fmwk: parsed,
	}, out)
}
