// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/runtime/docker/install"
)

func newPrepareCmd() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepares a workstation for debugging.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			spec := install.PersistentSpec{
				Name:          "jaeger",
				ContainerName: "fn-jaeger",
				Image:         "jaegertracing/all-in-one",
				Version:       "1.27",
				Ports: map[int]int{
					20000: 16686,
					20001: 14268,
				},
			}

			if err := spec.Ensure(ctx, console.Output(ctx, "jaeger")); err != nil {
				return err
			}

			w := console.Stdout(ctx)

			config, _ := dirs.Config()

			fmt.Fprintf(w, "\n  Jaeger listening on: http://localhost:20001/api/traces\n\n")
			fmt.Fprintf(w, "Consider updating %s/config.json with:\n\n", config)
			fmt.Fprintf(w, "  %q: %q\n\n", "jaeger_endpoint", "http://localhost:20001/api/traces")

			return nil
		}),
	}

	cmd.Flags().StringVar(&version, "version", "", "Which version to rehydrate.")

	return cmd
}
