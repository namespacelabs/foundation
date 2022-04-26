// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewBundlesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "bundles",
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "Lists stored action bundles.",
		Args:  cobra.MaximumNArgs(0),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			bundler, err := tasks.NewActionBundler()
			if err != nil {
				return err
			}
			bundles, err := bundler.ReadBundles()
			if err != nil {
				return err
			}
			if err := renderBundleTable(ctx, bundles, console.Stdout(ctx)); err != nil {
				return err
			}
			return nil
		}),
	}

	upload := &cobra.Command{
		Use:   "upload",
		Short: "Encrypts and uploads a bundle to foundation.",
		Args:  cobra.MaximumNArgs(0),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			bundler, err := tasks.NewActionBundler()
			if err != nil {
				return err
			}
			bundles, err := bundler.ReadBundles()
			if err != nil {
				return err
			}
			if err := renderBundleTable(ctx, bundles, console.Stdout(ctx)); err != nil {
				return err
			}

			lastBundleIdx := len(bundles) - 1
			done := console.EnterInputMode(ctx, fmt.Sprintf("\nEnter a bundle index between [%d - %d]: ", 0, lastBundleIdx))
			defer done()

			scanner := bufio.NewScanner(os.Stdin)
			idx := -1
			for scanner.Scan() {
				idx, err = strconv.Atoi(strings.TrimSpace(scanner.Text()))
				if err != nil || idx < 0 || idx > len(bundles) {
					fmt.Printf("\nplease enter a valid index between [%d - %d]: ", 0, lastBundleIdx)
					continue
				}
				break
			}
			// XXX Verify `bundle.EncryptTo` with a temp `io.Writer` while we
			// address lack of binary uploads in gRPC gateway as described in
			// https://github.com/grpc-ecosystem/grpc-gateway/issues/500.
			file, _ := dirs.CreateUserTemp("action-bundles", "actions-*.tar.gz.age")
			bundle := bundles[idx]
			if err := bundle.EncryptTo(ctx, file); err != nil {
				return err
			}
			fmt.Printf("\nSuccessfully wrote encrypted bundle to %s\n", file.Name())
			return nil
		}),
	}

	cmd.AddCommand(list)
	cmd.AddCommand(upload)

	return cmd
}

func renderBundleTable(ctx context.Context, bundles []*tasks.Bundle, w io.Writer) error {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Index", "Command", "Time"})
	table.SetBorder(false)

	var data [][]string
	for idx, bundle := range bundles {
		info, err := bundle.ReadInvocationInfo(ctx)
		if err != nil {
			return err
		}
		data = append(data, []string{fmt.Sprint(idx), info.Command, humanize.Time(bundle.Timestamp)})
	}
	table.SetHeaderColor(tablewriter.Colors{tablewriter.Bold, tablewriter.BgGreenColor},
		tablewriter.Colors{tablewriter.FgHiRedColor, tablewriter.Bold, tablewriter.BgBlackColor},
		tablewriter.Colors{tablewriter.BgCyanColor, tablewriter.FgWhiteColor})

	table.SetColumnColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiRedColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiBlackColor})

	table.AppendBulk(data)
	table.Render()
	return nil
}
