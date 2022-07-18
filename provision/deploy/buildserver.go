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
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/config"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var RunCodegen = true

func makePlan(ctx context.Context, server provision.Server, spec build.Spec) (build.Plan, error) {
	return tasks.Return(ctx, tasks.Action("fn.deploy.prepare-server-image").Scope(server.PackageName()),
		func(ctx context.Context) (build.Plan, error) {
			platforms, err := runtime.For(ctx, server.Env()).TargetPlatforms(ctx)
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
	serverPackage   schema.PackageName
	env             *schema.Environment
	stack           *schema.Stack
	moduleSrcs      []moduleAndFiles
	computedConfigs compute.Computable[*schema.ComputedConfigurations]

	compute.LocalScoped[fs.FS]
}

func (c *prepareServerConfig) Inputs() *compute.In {
	in := compute.Inputs().JSON("serverPackage", c.serverPackage).Proto("env", c.env).
		Proto("stack", c.stack).Computable("computedConfigs", c.computedConfigs)

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
	return tasks.Action("deploy.prepare-server-config").Arg("env", c.env.Name).Scope(c.serverPackage)
}

func (c *prepareServerConfig) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	var fragment []*schema.IngressFragment
	for _, entry := range c.stack.Entry {
		var err error
		fragment, err = runtime.ComputeIngress(ctx, c.env, entry, c.stack.Endpoint)
		if err != nil {
			return nil, err
		}
	}

	files := &memfs.FS{}
	if err := (config.DehydrateOpts{IncludeTextProto: true}).DehydrateTo(ctx, files, c.env, c.stack, fragment, compute.MustGetDepValue(deps, c.computedConfigs, "computedConfigs")); err != nil {
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

func prepareConfigImage(ctx context.Context, server provision.Server, stack *stack.Stack,
	computedConfigs compute.Computable[*schema.ComputedConfigurations]) oci.NamedImage {
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

	return oci.MakeImageFromScratch(fmt.Sprintf("config %s", server.PackageName()),
		oci.MakeLayer(fmt.Sprintf("config %s", server.PackageName()),
			&prepareServerConfig{
				serverPackage:   server.PackageName(),
				env:             server.Env().Proto(),
				stack:           stack.Proto(),
				computedConfigs: computedConfigs,
				moduleSrcs:      modulesSrcs,
			}))
}
