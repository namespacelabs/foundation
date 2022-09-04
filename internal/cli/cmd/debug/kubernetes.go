// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/prototext"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/vcluster"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/module"
)

func newKubernetesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "kubernetes",
	}

	envBound := "dev"
	systemInfo := &cobra.Command{
		Use:  "system-info",
		Args: cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, envBound)
			if err != nil {
				return err
			}

			k, err := kubernetes.NewFromEnv(ctx, env)
			if err != nil {
				return err
			}

			sysInfo, err := k.SystemInfo(ctx)
			if err != nil {
				return err
			}

			fmt.Fprintln(console.Stdout(ctx), prototext.Format(sysInfo))
			return nil
		}),
	}

	systemInfo.Flags().StringVar(&envBound, "env", envBound, "If specified, produce a env-bound sealed schema.")

	createVCluster := fncobra.CmdWithEnv(&cobra.Command{
		Use:  "create-vcluster",
		Args: cobra.ExactArgs(1),
	}, func(ctx context.Context, env provision.Env, args []string) error {
		hostConfig, err := client.ComputeHostConfig(env.Environment(), env.DevHost(), devhost.ByEnvironment(env.Environment()))
		if err != nil {
			return err
		}

		vc, err := compute.GetValue(ctx, vcluster.Create(nil, hostConfig, args[0]))
		if err != nil {
			return err
		}

		conn, err := vc.Access(ctx)
		if err != nil {
			return err
		}

		defer conn.Close()

		r, err := conn.Runtime(ctx)
		if err != nil {
			return err
		}

		pods, err := r.Client().CoreV1().Pods("kube-admin").List(ctx, v1.ListOptions{})
		if err != nil {
			return fnerrors.Wrapf(nil, err, "unable to list pods")
		}

		w := json.NewEncoder(console.Stderr(ctx))
		w.SetIndent("", "  ")
		return w.Encode(pods)
	})

	cmd.AddCommand(systemInfo)
	cmd.AddCommand(createVCluster)

	return cmd
}
