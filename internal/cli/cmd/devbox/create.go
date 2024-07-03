// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"
	"fmt"

	devboxv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/private/devbox"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func newCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create tag",
		Short: "Creates a devbox named 'tag'.",
		Args:  cobra.MinimumNArgs(1),
	}

	machineType := cmd.Flags().String("machine_type", "", "the path of the repository to work in")
	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return createDevbox(ctx, args[0], *machineType)
	})

	return cmd
}

func createDevbox(ctx context.Context, tag string, machineType string) error {
	devboxClient, err := getDevBoxClient(ctx)
	if err != nil {
		return err
	}
	devboxSpec := &devboxv1beta.DevBoxSpec{
		Tag:         tag,
		MachineType: machineType,
	}
	resp, err := devboxClient.CreateDevBox(ctx, &devboxv1beta.CreateDevBoxRequest{
		DevboxSpec: devboxSpec,
	})
	if err != nil {
		return err
	}
	fmt.Println(resp.GetDevbox().DevboxStatus)
	return nil
}
