// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/providers/aws/eks"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func newGenerateConfigCmd() *cobra.Command {
	cmd := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "kube-config",
		Short: "Generates a EKS kubeconfig.",
		Args:  cobra.ExactArgs(1),
	}, func(ctx context.Context, env provision.Env, args []string) error {
		s, err := eks.NewSession(ctx, env.Environment(), env.DevHost(), devhost.ByEnvironment(env.Environment()))
		if err != nil {
			return err
		}

		cluster, err := eks.DescribeCluster(ctx, s, args[0])
		if err != nil {
			return err
		}

		cfg, err := eks.Kubeconfig(cluster, env.Environment())
		if err != nil {
			return err
		}

		w := json.NewEncoder(console.Stdout(ctx))
		w.SetIndent("", "  ")
		return w.Encode(cfg)
	})

	return cmd
}
