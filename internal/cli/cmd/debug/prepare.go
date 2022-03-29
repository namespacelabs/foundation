// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime/docker/install"
)

func newPrepareCmd() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepares a workstation for development, for the most part.",
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

			fmt.Fprintf(w, "\n  Jaeger listening on: http://localhost:20001/api/traces\n")

			viper.Set("jaeger_endpoint", "http://localhost:20001/api/traces")

			if err := viper.WriteConfig(); err != nil {
				return err
			}

			fmt.Fprintf(w, "\n  (Updated configuration.)\n")

			return nil
		}),
	}

	cmd.Flags().StringVar(&version, "version", "", "Which version to rehydrate.")

	return cmd
}