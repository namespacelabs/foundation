// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"

	devboxv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/private/devbox"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/tui"
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [tag]...",
		Short: "Lists devboxes.",
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return listDevboxes(ctx, args)
	})

	return cmd
}

func listDevboxes(ctx context.Context, tagFilter []string) error {
	devboxClient, err := getDevBoxClient(ctx)
	if err != nil {
		return err
	}
	resp, err := devboxClient.ListDevBoxes(ctx, &devboxv1beta.ListDevBoxesRequest{
		TagFilter: tagFilter,
	})
	if err != nil {
		return err
	}

	if err := tableDevboxes(ctx, resp.GetDevBoxes()); err != nil {
		return err
	}
	return nil
}

const (
	tagColKey         = "tag"
	machineTypeColKey = "machineType"
	createdColKey     = "created"
	updatedColKey     = "updated"
	sshEndpointColKey = "sshEndpoint"
)

func tableDevboxes(ctx context.Context,
	devboxes []*devboxv1beta.DevBox) error {
	cols := []tui.Column{
		{Key: tagColKey, Title: "Tag", MinWidth: 5, MaxWidth: 24},
		{Key: machineTypeColKey, Title: "Machine type", MinWidth: 5, MaxWidth: 20},
		{Key: createdColKey, Title: "Created", MinWidth: 10, MaxWidth: 20},
		{Key: updatedColKey, Title: "Updated", MinWidth: 10, MaxWidth: 20},
	}

	rows := []tui.Row{}
	for _, devbox := range devboxes {
		created := devbox.GetDevboxStatus().GetCreatedAt().AsTime()
		updated := devbox.GetDevboxStatus().GetUpdatedAt().AsTime()
		row := tui.Row{
			tagColKey:         devbox.GetDevboxSpec().GetTag(),
			machineTypeColKey: devbox.GetDevboxSpec().GetMachineType(),
			createdColKey:     humanize.Time(created.Local()),
			updatedColKey:     humanize.Time(updated.Local()),
		}
		rows = append(rows, row)
	}

	return tui.StaticTable(ctx, cols, rows)
}
