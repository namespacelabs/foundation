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
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/config"
	"namespacelabs.dev/foundation/provision/parsed"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/tasks"
)

var RunCodegen = true

func MakePlan(ctx context.Context, rc runtime.Planner, server parsed.Server, spec build.Spec) (build.Plan, error) {
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
	env             planning.Context
	stack           *schema.Stack
	moduleSrcs      []moduleAndFiles
	computedConfigs compute.Computable[*schema.ComputedConfigurations]

	compute.LocalScoped[fs.FS]
}

func (c *prepareServerConfig) Inputs() *compute.In {
	in := compute.Inputs().
		Indigestible("planner", c.planner).
		JSON("serverPackage", c.serverPackage).
		Indigestible("env", c.env).
		Proto("stack", c.stack).
		Computable("computedConfigs", c.computedConfigs)

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

func prepareConfigImage(ctx context.Context, env planning.Context, planner runtime.Planner, server parsed.Server, stack *provision.Stack,
	computedConfigs compute.Computable[*schema.ComputedConfigurations]) oci.NamedImage {
	var modulesSrcs []moduleAndFiles
	for _, srcs := range server.SealedContext().Sources() {
		modulesSrcs = append(modulesSrcs, moduleAndFiles{
			moduleName: srcs.Module.ModuleName(),
			files:      srcs.Snapshot,
		})
	}

	sort.Slice(modulesSrcs, func(i, j int) bool {
		return strings.Compare(modulesSrcs[i].moduleName, modulesSrcs[j].moduleName) < 0
	})

	return oci.MakeImageFromScratch(fmt.Sprintf("config %s", server.PackageName()),
		oci.MakeLayer(fmt.Sprintf("config %s", server.PackageName()),
			&prepareServerConfig{
				planner:         planner,
				serverPackage:   server.PackageName(),
				env:             env,
				stack:           stack.Proto(),
				computedConfigs: computedConfigs,
				moduleSrcs:      modulesSrcs,
			}))
}
