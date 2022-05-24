package tools

import (
	"context"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/sdk/octant"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/workspace/module"
)

func newOctantCmd() *cobra.Command {
	var envRef = "dev"

	cmd := &cobra.Command{
		Use:   "octant",
		Short: "[Experimental] Run Octant, configured for the specified environment.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, envRef)
			if err != nil {
				return err
			}

			k8s, err := kubernetes.New(ctx, root.Workspace, root.DevHost, env.Proto())
			if err != nil {
				return err
			}

			bin, err := octant.EnsureSDK(ctx)
			if err != nil {
				return err
			}

			cfg := k8s.KubeConfig()

			done := console.EnterInputMode(ctx)
			defer done()

			octant := exec.CommandContext(ctx, string(bin), "--context="+cfg.Context, "--kubeconfig="+cfg.Config, "-n", cfg.Namespace)
			octant.Stdout = os.Stdout
			octant.Stderr = os.Stderr
			octant.Stdin = os.Stdin
			return octant.Run()
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to access (as defined in the workspace).")

	return cmd
}
