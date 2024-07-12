// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	devboxv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/private/devbox"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func newDescribeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe tag",
		Short: "Describe a devbox.",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return describeDevbox(ctx, args[0])
	})

	return cmd
}

func describeDevbox(ctx context.Context, tag string) error {
	devboxClient, err := getDevBoxClient(ctx)
	if err != nil {
		return err
	}

	devbox, err := getSingleDevbox(ctx, devboxClient, tag)
	if err != nil {
		return err
	}
	if fnapi.DebugApiResponse {
		fmt.Fprintf(console.Debug(ctx), "Response Body: %v\n", devbox)
	}

	stdout := console.Stdout(ctx)
	describeToWriter(stdout, devbox)

	return nil
}

func describeToWriter(out io.Writer, devbox *devboxv1beta.DevBox) {
	created := devbox.GetDevboxStatus().GetCreatedAt().AsTime()
	updated := devbox.GetDevboxStatus().GetUpdatedAt().AsTime()

	fmt.Fprintln(out, "Devbox ", devbox.GetDevboxSpec().GetTag())

	indented := indent(out)
	bodyWriter := tabwriter.NewWriter(indented, 0, 0, 2, ' ', 0)
	defer bodyWriter.Flush()

	fmt.Fprintln(bodyWriter, "Created:\t", humanize.Time(created.Local()))
	fmt.Fprintln(bodyWriter, "Updated:\t", humanize.Time(updated.Local()))
	fmt.Fprintln(bodyWriter, "Machine type:\t", devbox.GetDevboxSpec().GetMachineType())
	fmt.Fprintln(bodyWriter, "Region:\t", devbox.GetDevboxSpec().GetRegion())
	fmt.Fprintln(bodyWriter, "Base image:\t", devbox.GetDevboxSpec().GetBaseImageRef())
	fmt.Fprintln(bodyWriter, "SSH endpoint:\t", devbox.GetDevboxStatus().GetSshEndpoint())
}
