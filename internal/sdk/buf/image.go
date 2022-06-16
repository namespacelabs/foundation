// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buf

import (
	"context"

	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

const baseImgPkg schema.PackageName = "namespacelabs.dev/foundation/std/sdk/buf/baseimg"

func init() {
	workspace.ExtendNodeHook = append(workspace.ExtendNodeHook, func(context.Context, workspace.Packages, workspace.Location, *schema.Node) (*workspace.ExtendNodeHookResult, error) {
		return &workspace.ExtendNodeHookResult{
			LoadPackages: []schema.PackageName{baseImgPkg},
		}, nil
	})
	workspace.ExtendServerHook = append(workspace.ExtendServerHook, func(workspace.Location, *schema.Server) workspace.ExtendServerHookResult {
		return workspace.ExtendServerHookResult{
			Import: []schema.PackageName{baseImgPkg},
		}
	})
}

func Image(ctx context.Context, env ops.Environment, loader workspace.Packages) compute.Computable[oci.Image] {
	pkg, err := loader.LoadByName(ctx, baseImgPkg)
	if err != nil {
		return compute.Error[oci.Image](err)
	}

	platform, err := tools.HostPlatform(ctx)
	if err != nil {
		return compute.Error[oci.Image](err)
	}

	prep, err := binary.PlanImage(ctx, pkg, env, true, &platform)
	if err != nil {
		return compute.Error[oci.Image](err)
	}

	return prep.Image
}
