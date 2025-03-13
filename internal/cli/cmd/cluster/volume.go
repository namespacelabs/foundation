// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"sort"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewVolumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Volume-related activities.",
	}

	cmd.AddCommand(newListVolumesCmd())
	cmd.AddCommand(newReleaseVolumesCmd())

	return cmd
}

const (
	tagColKey        = "tag"
	sizeColKey       = "size"
	attachedToColKey = "attached_to"
	lastUsedColKey   = "last_used"
)

func newListVolumesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists the volumes for this workspace.",
		Args:  cobra.NoArgs,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		lst, err := api.ListVolumes(ctx, api.Methods)
		if err != nil {
			return err
		}

		latestIdx := map[string]api.Volume{}

		for _, vol := range lst.Volume {
			if vol == nil {
				continue
			}

			if curr, ok := latestIdx[vol.Tag]; ok && after(curr, *vol) {
				continue
			}

			latestIdx[vol.Tag] = *vol
		}

		cols := []tui.Column{
			{Key: tagColKey, Title: "Tag", MinWidth: 5, MaxWidth: 70},
			{Key: sizeColKey, Title: "Size", MinWidth: 3, MaxWidth: 20},
			{Key: attachedToColKey, Title: "Attached To", MinWidth: 5, MaxWidth: 20},
			{Key: lastUsedColKey, Title: "Last Used", MinWidth: 5, MaxWidth: 30},
		}

		vols := maps.Values(latestIdx)
		sort.Slice(vols, func(i, j int) bool {
			// Reverse sorting.
			return after(vols[i], vols[j])
		})

		rows := []tui.Row{}
		for _, vol := range vols {
			attachedTo := vol.AttachedTo
			if attachedTo == "" {
				attachedTo = "not attached"
			}
			row := tui.Row{
				tagColKey:        vol.Tag,
				sizeColKey:       fmt.Sprintf("%d GB", vol.SizeMb/1024),
				attachedToColKey: attachedTo,
			}

			if vol.LastAttachedAt != nil {
				row[lastUsedColKey] = humanize.Time(*vol.LastAttachedAt)
			}

			rows = append(rows, row)
		}

		return tui.StaticTable(ctx, cols, rows)
	})

	return cmd
}

func after(a, b api.Volume) bool {
	if a.AttachedTo != "" && b.AttachedTo == "" {
		return true
	}

	if a.AttachedTo == "" && b.AttachedTo != "" {
		return false
	}

	if a.LastAttachedAt == nil {
		return false
	}

	if b.LastAttachedAt == nil {
		return true
	}

	return a.LastAttachedAt.After(*b.LastAttachedAt)
}

func newReleaseVolumesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release volumes for a provided tag.",
		Args:  cobra.MaximumNArgs(1),
	}

	volumeId := cmd.Flags().String("id", "", "If set, only release the volume with this ID.")

	cmd.Flags().MarkHidden("id")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if len(args) > 0 && *volumeId != "" {
			return fnerrors.Newf("please provide either a volume tag or a volume id to release")
		}

		switch {
		case *volumeId != "":
			if err := api.DestroyVolume(ctx, api.Methods, *volumeId); err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "Released volume %s.\n", *volumeId)

		case len(args) == 1:
			tag := args[0]
			if err := api.DestroyVolumeByTag(ctx, api.Methods, tag); err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "Released volumes with tag %s.\n", tag)

		default:
			return fnerrors.Newf("please provide exactly one volume tag to release")
		}

		return nil
	})

	return cmd
}
