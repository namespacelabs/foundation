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

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var RunCodegen = true

func makePlan(ctx context.Context, server provision.Server, spec build.Spec) (plan build.Plan, err error) {
	platforms, err := runtime.For(ctx, server.Env()).TargetPlatforms(ctx)
	if err != nil {
		return plan, err
	}

	err = tasks.Action("fn.deploy.prepare-server-image").
		Scope(server.PackageName()).
		Arg("platforms", platforms).
		Run(ctx, func(ctx context.Context) error {
			var ws build.Workspace
			if RunCodegen {
				ws = codegenWorkspace{server}
			} else {
				ws = server.Module()
			}

			plan = build.Plan{
				SourceLabel:   fmt.Sprintf("Server %s", server.PackageName()),
				SourcePackage: server.PackageName(),
				Spec:          spec,
				Workspace:     ws,
				Platforms:     platforms,
			}

			return err
		})
	return
}

type prepareServerConfig struct {
	env             *schema.Environment
	stack           *schema.Stack
	moduleSrcs      []moduleAndFiles
	computedConfigs compute.Computable[*schema.ComputedConfigurations]

	compute.LocalScoped[fs.FS]
}

func (c *prepareServerConfig) Inputs() *compute.In {
	in := compute.Inputs().Proto("env", c.env).Proto("stack", c.stack).Computable("computedConfigs", c.computedConfigs)

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
	var fragment []*schema.IngressFragment
	for _, entry := range c.stack.Entry {
		deferred, err := runtime.ComputeIngress(ctx, c.env, entry, c.stack.Endpoint)
		if err != nil {
			return nil, err
		}
		for _, d := range deferred {
			fragment = append(fragment, d.WithoutAllocation())
		}
	}

	messages, err := protos.SerializeMultiple(
		c.env,
		c.stack,
		&schema.IngressFragmentList{IngressFragment: fragment},
		compute.MustGetDepValue(deps, c.computedConfigs, "computedConfigs"),
	)
	if err != nil {
		return nil, err
	}

	env := messages[0]
	stack := messages[1]
	ingress := messages[2]
	computedConfigs := messages[3]

	files := &memfs.FS{}

	for _, f := range []fnfs.File{
		{Path: "config/env.textpb", Contents: env.Text},
		{Path: "config/env.binarypb", Contents: env.Binary},
		{Path: "config/stack.textpb", Contents: stack.Text},
		{Path: "config/stack.binarypb", Contents: stack.Binary},
		{Path: "config/ingress.textpb", Contents: ingress.Text},
		{Path: "config/ingress.binarypb", Contents: ingress.Binary},
		{Path: "config/computed_configs.textpb", Contents: computedConfigs.Text},
		{Path: "config/computed_configs.binarypb", Contents: computedConfigs.Binary},
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

func prepareConfigImage(ctx context.Context, server provision.Server, stack *stack.Stack,
	computedConfigs compute.Computable[*schema.ComputedConfigurations]) compute.Computable[oci.Image] {
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
				env:             server.Env().Proto(),
				stack:           stack.Proto(),
				computedConfigs: computedConfigs,
				moduleSrcs:      modulesSrcs,
			}))
}
