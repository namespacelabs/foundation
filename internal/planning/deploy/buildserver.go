// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"
	"io/fs"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/config"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

var RunCodegen = true

func MakePlan(ctx context.Context, rc runtime.Planner, server planning.Server, spec build.Spec) (build.Plan, error) {
	return tasks.Return(ctx, tasks.Action("fn.deploy.prepare-server-image").Scope(server.PackageName()),
		func(ctx context.Context) (build.Plan, error) {
			platforms, err := rc.TargetPlatforms(ctx)
			if err != nil {
				return build.Plan{}, err
			}

			tasks.Attachments(ctx).AddResult("platforms", platforms)

			var ws build.Workspace
			if RunCodegen {
				ws = codegenWorkspace{server}
			} else {
				ws = server.Module()
			}

			return build.Plan{
				SourceLabel:   fmt.Sprintf("Server %s", server.PackageName()),
				SourcePackage: server.PackageName(),
				BuildKind:     storage.Build_SERVER,
				Spec:          spec,
				Workspace:     ws,
				Platforms:     platforms,
			}, nil
		})
}

type prepareServerConfig struct {
	planner         runtime.Planner
	serverPackage   schema.PackageName
	env             cfg.Context
	stack           *schema.Stack
	computedConfigs compute.Computable[*schema.ComputedConfigurations]

	compute.LocalScoped[fs.FS]
}

func (c *prepareServerConfig) Inputs() *compute.In {
	return compute.Inputs().
		Indigestible("planner", c.planner).
		JSON("serverPackage", c.serverPackage).
		Indigestible("env", c.env).
		Proto("stack", c.stack).
		Computable("computedConfigs", c.computedConfigs)
}

func (c *prepareServerConfig) Action() *tasks.ActionEvent {
	return tasks.Action("deploy.prepare-server-config").Arg("env", c.env.Environment().Name).Scope(c.serverPackage)
}

func (c *prepareServerConfig) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	var fragment []*schema.IngressFragment
	for _, entry := range c.stack.Entry {
		var err error
		fragment, err = runtime.ComputeIngress(ctx, c.env, c.planner, entry, c.stack.Endpoint)
		if err != nil {
			return nil, err
		}
	}

	// XXX These should be scoped down to only the configs provided by this server.
	computedConfigs := compute.MustGetDepValue(deps, c.computedConfigs, "computedConfigs")

	files := &memfs.FS{}
	if err := (config.DehydrateOpts{IncludeTextProto: true}).DehydrateTo(ctx, c.env.Environment(), c.stack, fragment, computedConfigs, files); err != nil {
		return nil, err
	}

	return files, nil
}

func prepareConfigImage(ctx context.Context, env cfg.Context, planner runtime.Planner, server planning.Server, stack *planning.Stack,
	computedConfigs compute.Computable[*schema.ComputedConfigurations]) oci.NamedImage {

	return oci.MakeImageFromScratch(fmt.Sprintf("config %s", server.PackageName()),
		oci.MakeLayer(fmt.Sprintf("config %s", server.PackageName()),
			&prepareServerConfig{
				planner:         planner,
				serverPackage:   server.PackageName(),
				env:             env,
				stack:           stack.Proto(),
				computedConfigs: computedConfigs,
			}))
}
