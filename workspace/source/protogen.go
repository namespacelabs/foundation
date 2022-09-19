// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"fmt"
	"io/fs"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/sdk/buf"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	srcprotos "namespacelabs.dev/foundation/workspace/source/protos"
)

func RegisterGraphHandlers() {
	ops.RegisterHandlerFunc(func(ctx context.Context, _ *schema.SerializedInvocation, op *OpMultiProtoGen) (*ops.HandleResult, error) {
		request := map[schema.Framework]*srcprotos.FileDescriptorSetAndDeps{}
		var errs []error
		for _, entry := range op.Protos {
			var err error
			request[entry.Framework], err = srcprotos.Merge(entry.Protos...)
			errs = append(errs, err)
		}

		if mergeErr := multierr.New(errs...); mergeErr != nil {
			return nil, mergeErr
		}

		module, err := ops.Get(ctx, pkggraph.MutableModuleInjection)
		if err != nil {
			return nil, err
		}

		config, err := ops.Get(ctx, ops.ConfigurationInjection)
		if err != nil {
			return nil, err
		}

		if err := generateProtoSrcs(ctx, config, request, module.ReadWriteFS()); err != nil {
			return nil, err
		}

		return nil, nil
	})

	ops.Compile[*OpProtoGen](func(ctx context.Context, inputs []*schema.SerializedInvocation) ([]*schema.SerializedInvocation, error) {
		module, err := ops.Get(ctx, pkggraph.MutableModuleInjection)
		if err != nil {
			return nil, err
		}

		loader, err := ops.Get(ctx, pkggraph.PackageLoaderInjection)
		if err != nil {
			return nil, err
		}

		requests := map[schema.Framework][]*srcprotos.FileDescriptorSetAndDeps{}
		for _, input := range inputs {
			msg := &OpProtoGen{}
			if err := input.Impl.UnmarshalTo(msg); err != nil {
				return nil, err
			}

			loc, err := loader.Resolve(ctx, schema.PackageName(msg.PackageName))
			if err != nil {
				return nil, err
			}

			if loc.Module.ModuleName() != module.ModuleName() {
				return nil, fnerrors.BadInputError("%s: can't perform codegen for packages in %q", module.ModuleName(), loc.Module.ModuleName())
			}

			requests[msg.Framework] = append(requests[msg.Framework], msg.Protos)
		}

		multi := &OpMultiProtoGen{}
		for fmwk, protos := range requests {
			multi.Protos = append(multi.Protos, &OpMultiProtoGen_ProtosByFramework{
				Framework: fmwk,
				Protos:    protos,
			})
		}
		slices.SortFunc(multi.Protos, func(a, b *OpMultiProtoGen_ProtosByFramework) bool {
			return int(a.Framework) < int(b.Framework)
		})

		return []*schema.SerializedInvocation{
			{
				Impl: protos.WrapAnyOrDie(multi),
			},
		}, nil
	})
}

func generateProtoSrcs(ctx context.Context, env planning.Configuration, request map[schema.Framework]*srcprotos.FileDescriptorSetAndDeps, out fnfs.ReadWriteFS) error {
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

	return generateProtoSrcs(ctx, env.Configuration(), map[schema.Framework]*srcprotos.FileDescriptorSetAndDeps{
		fmwk: parsed,
	}, out)
}
