// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package eks

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/providers/aws/eks"
	"namespacelabs.dev/foundation/std/cfg"
)

func newGenerateConfigCmd() *cobra.Command {
	cmd := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "kube-config",
		Short: "Generates a EKS kubeconfig.",
		Args:  cobra.ExactArgs(1),
	}, func(ctx context.Context, env cfg.Context, args []string) error {
		s, err := eks.NewSession(ctx, env.Configuration())
		if err != nil {
			return err
		}

		cluster, err := eks.DescribeCluster(ctx, s, args[0])
		if err != nil {
			return err
		}

		cfg, err := eks.Kubeconfig(cluster, env.Configuration().EnvKey())
		if err != nil {
			return err
		}

		w := json.NewEncoder(console.Stdout(ctx))
		w.SetIndent("", "  ")
		return w.Encode(cfg)
	})

	return cmd
}
