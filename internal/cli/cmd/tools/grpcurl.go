// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/workspace/compute"
)

func newGRPCurlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "grpcurl",
		Short:              "Run grpcurl.",
		DisableFlagParsing: true,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			rt := tools.Impl()

			res, err := compute.Get(ctx, oci.ResolveImage("fullstorydev/grpcurl:v1.8.5", rt.HostPlatform()))
			if err != nil {
				return err
			}

			return tools.Impl().Run(ctx, rtypes.RunToolOpts{
				IO:                rtypes.StdIO(ctx),
				UseHostNetworking: true,
				RunBinaryOpts: rtypes.RunBinaryOpts{
					WorkingDir: "/",
					Image:      res.Value.(v1.Image),
					Args:       args,
				},
			})
		}),
	}

	return cmd
}