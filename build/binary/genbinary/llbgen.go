// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package genbinary

import (
	"bytes"
	"context"
	"fmt"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func LLBBinary(packageName schema.PackageName, module *workspace.Module, bin build.Spec) build.Spec {
	return llbBinary{packageName, module, bin}
}

type llbBinary struct {
	packageName schema.PackageName
	module      *workspace.Module
	bin         build.Spec
}

func (l llbBinary) BuildImage(ctx context.Context, env ops.Environment, conf build.Configuration) (compute.Computable[oci.Image], error) {
	hostPlatform, err := tools.HostPlatform(ctx)
	if err != nil {
		return nil, err
	}

	bin, err := l.bin.BuildImage(ctx, env, build.NewBuildTarget(&hostPlatform).WithWorkspace(l.module))
	if err != nil {
		return nil, err
	}

	action := tasks.Action("binary.llbgen").Scope(l.packageName)

	if conf.TargetPlatform() != nil {
		action = action.Arg("platform", devhost.FormatPlatform(*conf.TargetPlatform()))
	}

	return compute.Map(action, compute.Inputs().Computable("bin", bin).JSON("platform", conf.TargetPlatform()), compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (oci.Image, error) {
			binImage := compute.MustGetDepValue(deps, bin, "bin")

			var targetPlatform string
			if conf.TargetPlatform() != nil {
				targetPlatform = devhost.FormatPlatform(*conf.TargetPlatform())
			}

			var serializedLLB bytes.Buffer

			var run rtypes.RunToolOpts
			run.ImageName = fmt.Sprintf("%s/llbgen", l.packageName)
			run.AllocateTTY = false
			run.NoNetworking = true
			run.IO.Stdout = &serializedLLB
			run.IO.Stderr = console.Output(ctx, "llbgen")
			run.WorkingDir = "/"
			run.Image = binImage
			// XXX security user id
			run.Command = []string{"/" + binary.LLBGenBinaryName}
			run.Env = []*schema.BinaryConfig_EnvEntry{
				{Name: "TARGET_PLATFORM", Value: targetPlatform},
			}

			if err := tools.Run(ctx, run); err != nil {
				return nil, fnerrors.UserError(nil, "failed to call llbgen :%w", err)
			}

			def, err := llb.ReadFrom(bytes.NewReader(serializedLLB.Bytes()))
			if err != nil {
				return nil, err
			}

			return compute.GetValue(ctx, buildkit.DefinitionToImage(env, conf, def))
		}), nil
}

func (l llbBinary) PlatformIndependent() bool { return false }
