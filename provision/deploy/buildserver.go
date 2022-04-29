// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func makePlan(ctx context.Context, server provision.Server, spec build.Spec) (plan build.Plan, err error) {
	platforms, err := runtime.For(ctx, server.Env()).TargetPlatforms(ctx)
	if err != nil {
		return plan, err
	}

	err = tasks.Action("fn.deploy.prepare-server-image").
		Scope(server.PackageName()).
		Arg("platforms", platforms).
		Run(ctx, func(ctx context.Context) error {
			plan = build.Plan{
				SourceLabel:   fmt.Sprintf("Server %s", server.PackageName()),
				SourcePackage: server.PackageName(),
				Spec:          spec,
				Workspace:     codegenWorkspace{server},
				Platforms:     platforms,
			}

			return err
		})
	return
}

type prepareServerConfig struct {
	env        *schema.Environment
	stack      *schema.Stack
	moduleSrcs []moduleAndFiles

	compute.LocalScoped[fs.FS]
}

func (c *prepareServerConfig) Inputs() *compute.In {
	in := compute.Inputs().Proto("env", c.env).Proto("stack", c.stack)

	return in.Marshal("moduleSrcs", func(ctx context.Context, w io.Writer) error {
		for _, m := range c.moduleSrcs {
			digest, err := fscache.ComputeDigest(ctx, m.files)
			if err != nil {
				return err
			}

			fmt.Fprintln(w, m.moduleName, ":", digest.String())
		}

		return nil
	})
}

func (c *prepareServerConfig) Action() *tasks.ActionEvent {
	return tasks.Action("deploy.prepare-server-config")
}

func (c *prepareServerConfig) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	envtext, err := prototext.Marshal(c.env)
	if err != nil {
		return nil, err
	}

	envpb, err := proto.MarshalOptions{Deterministic: true}.Marshal(c.env)
	if err != nil {
		return nil, err
	}

	stackProto := c.stack
	stacktext, err := prototext.Marshal(stackProto)
	if err != nil {
		return nil, err
	}

	stackpb, err := proto.MarshalOptions{Deterministic: true}.Marshal(stackProto)
	if err != nil {
		return nil, err
	}

	files := &memfs.FS{}

	for _, f := range []fnfs.File{
		{Path: "config/env.textpb", Contents: envtext},
		{Path: "config/env.binarypb", Contents: envpb},
		{Path: "config/stack.textpb", Contents: stacktext},
		{Path: "config/stack.binarypb", Contents: stackpb},
	} {
		if err := fnfs.WriteFile(ctx, files, f.Path, f.Contents, 0644); err != nil {
			return nil, err
		}
	}

	for _, m := range c.moduleSrcs {
		if err := fnfs.CopyTo(ctx, files, "config/srcs/"+m.moduleName+"/", m.files); err != nil {
			return nil, err
		}
	}

	return files, nil
}

type moduleAndFiles struct {
	moduleName string
	files      fs.FS
}

func prepareConfigImage(ctx context.Context, server provision.Server, stack *stack.Stack) compute.Computable[oci.Image] {
	var modulesSrcs []moduleAndFiles
	for _, srcs := range server.Env().Sources() {
		modulesSrcs = append(modulesSrcs, moduleAndFiles{
			moduleName: srcs.Module.ModuleName(),
			files:      srcs.Snapshot,
		})
	}

	sort.Slice(modulesSrcs, func(i, j int) bool {
		return strings.Compare(modulesSrcs[i].moduleName, modulesSrcs[j].moduleName) < 0
	})

	return oci.MakeImage(
		oci.Scratch(),
		oci.MakeLayer("config",
			&prepareServerConfig{
				env:        server.Env().Proto(),
				stack:      stack.Proto(),
				moduleSrcs: modulesSrcs,
			}))
}
