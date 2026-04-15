// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

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
	tagColKey      = "tag"
	sizeColKey     = "size"
	lastUsedColKey = "last_used"
)

func newListVolumesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists the volumes for this workspace.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		lst, err := api.ListVolumes(ctx, api.Methods)
		if err != nil {
			return err
		}

		type tagGroup struct {
			tag              string
			latestUnattached *api.Volume // unattached volume with highest LastAttachedAt
			sizeMb           uint32
			inUse            bool
			lastAttachedAt   *time.Time // highest LastAttachedAt across all volumes
		}

		groups := map[string]*tagGroup{}

		for _, vol := range lst.Volume {
			if vol == nil {
				continue
			}

			g, ok := groups[vol.Tag]
			if !ok {
				g = &tagGroup{tag: vol.Tag, sizeMb: vol.SizeMb}
				groups[vol.Tag] = g
			}

			if vol.AttachedTo != "" {
				g.inUse = true
			}

			if vol.LastAttachedAt != nil && (g.lastAttachedAt == nil || vol.LastAttachedAt.After(*g.lastAttachedAt)) {
				g.lastAttachedAt = vol.LastAttachedAt
			}

			if vol.AttachedTo == "" {
				if g.latestUnattached == nil || (vol.LastAttachedAt != nil && (g.latestUnattached.LastAttachedAt == nil || vol.LastAttachedAt.After(*g.latestUnattached.LastAttachedAt))) {
					g.latestUnattached = vol
				}
			}
		}

		tags := maps.Values(groups)
		sort.Slice(tags, func(i, j int) bool {
			if tags[i].inUse != tags[j].inUse {
				return tags[i].inUse
			}
			if tags[i].lastAttachedAt == nil {
				return false
			}
			if tags[j].lastAttachedAt == nil {
				return true
			}
			return tags[i].lastAttachedAt.After(*tags[j].lastAttachedAt)
		})

		if *output == "json" {
			type volumeJSON struct {
				Tag            string     `json:"tag"`
				UsedMb         *uint32    `json:"used_mb,omitempty"`
				SizeMb         uint32     `json:"size_mb"`
				LastAttachedAt *time.Time `json:"last_attached_at,omitempty"`
			}

			entries := make([]volumeJSON, 0, len(tags))
			for _, g := range tags {
				e := volumeJSON{
					Tag:            g.tag,
					SizeMb:         g.sizeMb,
					LastAttachedAt: g.lastAttachedAt,
				}
				if g.latestUnattached != nil {
					e.UsedMb = &g.latestUnattached.UsedMb
				}
				entries = append(entries, e)
			}

			stdout := console.Stdout(ctx)
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(entries); err != nil {
				return fnerrors.InternalError("failed to encode volume list as JSON output: %w", err)
			}
			return nil
		}

		cols := []tui.Column{
			{Key: tagColKey, Title: "Tag", MinWidth: 5, MaxWidth: 70},
			{Key: sizeColKey, Title: "Size (Used / Total)", MinWidth: 20, MaxWidth: 30},
			{Key: lastUsedColKey, Title: "Last Used", MinWidth: 5, MaxWidth: 30},
		}

		type fmtTag struct {
			g           *tagGroup
			used, total string
		}

		formatted := make([]fmtTag, 0, len(tags))
		maxUsedLen, maxTotalLen := 0, 0
		for _, g := range tags {
			var used string
			if g.latestUnattached != nil {
				used = humanize.IBytes(uint64(g.latestUnattached.UsedMb) * 1024 * 1024)
			} else {
				used = "-"
			}
			total := fmt.Sprintf("%d GiB", g.sizeMb/1024)
			formatted = append(formatted, fmtTag{g: g, used: used, total: total})
			if len(used) > maxUsedLen {
				maxUsedLen = len(used)
			}
			if len(total) > maxTotalLen {
				maxTotalLen = len(total)
			}
		}

		rows := []tui.Row{}
		for _, f := range formatted {
			row := tui.Row{
				tagColKey:  f.g.tag,
				sizeColKey: fmt.Sprintf("%*s / %*s", maxUsedLen, f.used, maxTotalLen, f.total),
			}
			if f.g.inUse {
				row[lastUsedColKey] = "In use"
			} else if f.g.lastAttachedAt != nil {
				row[lastUsedColKey] = humanize.Time(*f.g.lastAttachedAt)
			}
			rows = append(rows, row)
		}

		return tui.StaticTable(ctx, cols, rows)
	})

	return cmd
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
