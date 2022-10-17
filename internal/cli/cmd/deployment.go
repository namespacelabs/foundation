// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewDeploymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deployment",
		Short: "Manage deployments for a specified environment.",
	}

	defaultYes := false
	wait := true

	remove := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "remove --env {dev|staging|prod}",
		Short: "Removes all deployment assets associated with the specified environment.",
		Args:  cobra.NoArgs,
	}, func(ctx context.Context, env cfg.Context, args []string) error {
		if !defaultYes {
			if err := checkDelete(ctx, env.Environment().Name, true); err != nil {
				return err
			}
		}

		cluster, err := runtime.NamespaceFor(ctx, env)
		if err != nil {
			return err
		}

		removed, err := cluster.DeleteRecursively(ctx, wait)
		if removed {
			fmt.Fprintln(console.Stdout(ctx), "Resources removed.")
		} else if err == nil {
			fmt.Fprintln(console.Stdout(ctx), "Nothing to remove.")
		}

		return err
	})

	remove.Flags().BoolVar(&defaultYes, "yes", defaultYes, "If set to true, assume yes on prompts.")
	remove.Flags().BoolVar(&wait, "wait", wait, "If set to true, waits until all resources are removed before returning.")

	removeAll := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "remove-all",
		Short: "Removes all deployment assets associated with the specified environment.",
		Args:  cobra.NoArgs,
	}, func(ctx context.Context, env cfg.Context, args []string) error {
		if !defaultYes {
			if err := checkDelete(ctx, env.Environment().Name, false); err != nil {
				return err
			}
		}

		cluster, err := runtime.ClusterFor(ctx, env)
		if err != nil {
			return err
		}

		if _, err := cluster.DeleteAllRecursively(ctx, wait, console.Stdout(ctx)); err != nil {
			return err
		}

		return nil
	})

	removeAll.Flags().BoolVar(&defaultYes, "yes", defaultYes, "If set to true, assume yes on prompts.")
	removeAll.Flags().BoolVar(&wait, "wait", wait, "If set to true, waits until all resources are removed before returning.")

	cmd.AddCommand(remove)
	cmd.AddCommand(removeAll)

	return cmd
}

func checkDelete(ctx context.Context, env string, single bool) error {
	var title string
	if single {
		title = fmt.Sprintf("Remove %s's deployment?", env)
	} else {
		title = "Remove all Namespace-managed deployments?"
	}

	written, err := tui.Ask(ctx, title,
		fmt.Sprintf("Removing a deployment is a destructive operation -- any data that is a part of the environment will not be recoverable.\n\nPlease type %q to confirm you'd like to remove all of its resources.", env),
		"")
	if err != nil {
		return err
	}

	if written != env {
		return fnerrors.New("environment name didn't match, canceling")
	}

	return nil
}
