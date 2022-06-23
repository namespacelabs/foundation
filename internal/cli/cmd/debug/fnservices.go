// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/provision"
)

func NewFnServicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "fnservices",
		Short:  "Namespace services-related activities (internal only).",
		Hidden: true,
	}

	robotLogin := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "robot-login",
		Short: "Attempts to login as a robot.",
		Args:  cobra.ExactArgs(1),
	}, func(ctx context.Context, env provision.Env, args []string) error {
		accessToken, err := tui.Ask(ctx, "Which Access Token would you like to use today?", "That would be a Github access token.", "access token")
		if err != nil {
			return err
		}

		userAuth, err := fnapi.RobotLogin(ctx, args[0], accessToken)
		if err != nil {
			return err
		}

		w := json.NewEncoder(console.Stdout(ctx))
		w.SetIndent("", "  ")
		return w.Encode(userAuth)
	})

	var fqdn, target string

	mapAddr := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "naming-map",
		Short: "Maps a FQDN within Namespace Cloud's scope to a particular target (e.g. CNAME, or IP address).",
		Args:  cobra.NoArgs,
	}, func(ctx context.Context, env provision.Env, args []string) error {
		return fnapi.Map(ctx, fqdn, target)
	})

	mapAddr.Flags().StringVar(&fqdn, "fqdn", "", "Fully qualified domain.")
	mapAddr.Flags().StringVar(&target, "target", "", "Target address.")

	_ = mapAddr.MarkFlagRequired("fqdn")
	_ = mapAddr.MarkFlagRequired("target")

	var org string

	allocateName := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "naming-allocate-name",
		Short: "Allocates a TLS certificate within Namespace Cloud's scope.",
		Args:  cobra.NoArgs,
	}, func(ctx context.Context, env provision.Env, args []string) error {
		nr, err := fnapi.RawAllocateName(ctx, fnapi.AllocateOpts{
			FQDN: fqdn,
			Org:  org,
		})
		if err != nil {
			return err
		}

		w := json.NewEncoder(console.Stdout(ctx))
		w.SetIndent("", "  ")
		return w.Encode(nr)
	})

	allocateName.Flags().StringVar(&fqdn, "fqdn", "", "Fully qualified domain.")
	allocateName.Flags().StringVar(&org, "org", "", "Organization to identify as.")

	_ = allocateName.MarkFlagRequired("fqdn")

	cmd.AddCommand(robotLogin)
	cmd.AddCommand(mapAddr)
	cmd.AddCommand(allocateName)

	return cmd
}
