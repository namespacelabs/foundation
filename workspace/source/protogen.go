// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"io/fs"
	"sync"

	"namespacelabs.dev/foundation/internal/artifacts/fsops"
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

type ProtosOpts struct {
	Framework schema.Framework
}

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

	mod := &perModuleGen{}
	mod.descriptors.add(msg.Framework, msg.Protos)
	return nil, generateProtoSrcs(ctx, env, mod, wenv.OutputFS())
}

func (statefulGen) StartSession(ctx context.Context, env ops.Environment) ops.Session[*OpProtoGen] {
	wenv, ok := env.(workspace.MutableWorkspaceEnvironment)
	if !ok {
		// An error will then be returned in Close().
		wenv = nil
	}

	return &multiGen{ctx: ctx, wenv: wenv}
}

type multiGen struct {
	ctx  context.Context
	wenv workspace.MutableWorkspaceEnvironment

	mu    sync.Mutex
	locs  []workspace.Location
	opts  []ProtosOpts
	files []*protos.FileDescriptorSetAndDeps
}

func (m *multiGen) Handle(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, msg *OpProtoGen) (*ops.HandleResult, error) {
	wenv, ok := env.(workspace.Packages)
	if !ok {
		return nil, fnerrors.New("workspace.Packages required")
	}

	loc, err := wenv.Resolve(ctx, schema.PackageName(msg.PackageName))
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.locs = append(m.locs, loc)
	m.opts = append(m.opts, ProtosOpts{
		Framework: msg.Framework,
	})
	m.files = append(m.files, msg.Protos)

	return nil, nil
}

type perLanguageDescriptors struct {
	descriptorsMap map[schema.Framework][]*protos.FileDescriptorSetAndDeps
}

func (p *perLanguageDescriptors) add(framework schema.Framework, fileDescSet *protos.FileDescriptorSetAndDeps) {
	if p.descriptorsMap == nil {
		p.descriptorsMap = map[schema.Framework][]*protos.FileDescriptorSetAndDeps{}
	}

	descriptors, ok := p.descriptorsMap[framework]
	if !ok {
		descriptors = []*protos.FileDescriptorSetAndDeps{}
	}

	descriptors = append(descriptors, fileDescSet)
	p.descriptorsMap[framework] = descriptors
}

type perModuleGen struct {
	root        *workspace.Module
	descriptors perLanguageDescriptors
}

func ensurePerModule(mods []*perModuleGen, root *workspace.Module) ([]*perModuleGen, *perModuleGen) {
	for _, mod := range mods {
		if mod.root.Abs() == root.Abs() {
			return mods, mod
		}
	}

	mod := &perModuleGen{root: root}
	return append(mods, mod), mod
}

func (m *multiGen) Commit() error {
	if m.wenv == nil {
		return fnerrors.New("WorkspaceEnvironment required")
	}

	var mods []*perModuleGen
	var mod *perModuleGen

	m.mu.Lock()

	for k := range m.locs {
		mods, mod = ensurePerModule(mods, m.locs[k].Module)
		mod.descriptors.add(m.opts[k].Framework, m.files[k])
	}

	m.mu.Unlock()

	var errs []error
	for _, mod := range mods {
		if err := generateProtoSrcs(m.ctx, m.wenv, mod, m.wenv.OutputFS()); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
}

func generateProtoSrcs(ctx context.Context, env ops.Environment, mod *perModuleGen, out fnfs.ReadWriteFS) error {
	var fsys []compute.Computable[fs.FS]

	for framework, descriptors := range mod.descriptors.descriptorsMap {
		if len(descriptors) != 0 {
			srcs, err := buf.MakeProtoSrcs(ctx, env, protos.Merge(descriptors...), framework)
			if err != nil {
				return err
			}
			fsys = append(fsys, srcs)
		}
	}

	if len(fsys) == 0 {
		return nil
	}

	merged, err := compute.Get(ctx, fsops.Merge(fsys))
	if err != nil {
		return err
	}

	if err := fnfs.WriteFSToWorkspace(ctx, console.Stdout(ctx), out, merged.Value); err != nil {
		return err
	}

	return nil
}

func GenProtosAtPaths(ctx context.Context, env ops.Environment, src fs.FS, fmwk schema.Framework, paths []string, out fnfs.ReadWriteFS) error {
	parsed, err := protos.Parse(src, paths)
	if err != nil {
		return err
	}

	mod := &perModuleGen{}
	mod.descriptors.add(fmwk, parsed)

	return generateProtoSrcs(ctx, env, mod, out)
}
