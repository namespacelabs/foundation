// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
)

func newEnsureCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "ensure tag",
		Short:  "Ensure that an instance is running for the devbox.",
		Args:   cobra.MinimumNArgs(1),
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return ensureDevbox(ctx, args[0])
	})

	return cmd
}

func ensureDevbox(ctx context.Context, tag string) error {
	devboxClient, err := getDevBoxClient(ctx)
	if err != nil {
		return err
	}

	instance, err := doEnsureDevbox(ctx, devboxClient, tag)
	if err != nil {
		return err
	}

	stdout := console.Stdout(ctx)
	bodyWriter := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	defer bodyWriter.Flush()

	fmt.Fprintln(bodyWriter, "Regional Instance ID:\t", instance.regionalInstanceId)
	fmt.Fprintln(bodyWriter, "Regional SSH endpoint:\t", instance.regionalSshEndpoint)

	return nil
}
