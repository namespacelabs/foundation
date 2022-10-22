// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package debug

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

func newDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Downloads an URL.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			var downloads []compute.Computable[bytestream.ByteStream]

			for _, arg := range args {
				var artifact artifacts.Reference
				parts := strings.SplitN(arg, "@", 2)
				artifact.URL = parts[0]
				if len(parts) == 2 {
					var err error
					artifact.Digest, err = schema.ParseDigest(parts[1])
					if err != nil {
						return err
					}
				}

				downloads = append(downloads, download.URL(artifact))
			}

			_, err := compute.Get(ctx, compute.Collect(tasks.Action("download-all"), downloads...))
			if err != nil {
				return err
			}

			return nil
		}),
	}

	return cmd
}
