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
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewBundlesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "command-history",
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "Lists previous foundation command invocations.",
		Args:  cobra.MaximumNArgs(0),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			bundler := tasks.NewActionBundler()

			bundles, err := bundler.ReadBundles()
			if err != nil {
				return err
			}
			validBundles := bundlesWithInvocationInfo(ctx, bundles)
			if err := renderBundleTable(ctx, validBundles, console.Stdout(ctx)); err != nil {
				return err
			}
			return nil
		}),
	}

	upload := &cobra.Command{
		Use:   "upload",
		Short: "Encrypts and uploads a command bundle to foundation.",
		Args:  cobra.MaximumNArgs(0),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			bundler := tasks.NewActionBundler()

			bundles, err := bundler.ReadBundles()
			if err != nil {
				return err
			}
			validBundles := bundlesWithInvocationInfo(ctx, bundles)
			if err := renderBundleTable(ctx, validBundles, console.Stdout(ctx)); err != nil {
				return err
			}
			idx, err := promptForBundleIdx(ctx, 1, len(validBundles))
			if err != nil {
				return err
			}
			// XXX Verify `bundle.EncryptTo` with a temp `io.Writer` while we
			// address lack of binary uploads in gRPC gateway as described in
			// https://github.com/grpc-ecosystem/grpc-gateway/issues/500.
			file, _ := dirs.CreateUserTemp("action-bundles", "actions-*.tar.gz.age")
			bundleInfo := validBundles[idx-1]
			if err := bundleInfo.bundle.EncryptTo(ctx, file); err != nil {
				return err
			}
			fmt.Fprintf(console.Stdout(ctx), "\nSuccessfully wrote encrypted bundle to %s\n", file.Name())
			return nil
		}),
	}

	cmd.AddCommand(list)
	cmd.AddCommand(upload)

	return cmd
}

func promptForBundleIdx(ctx context.Context, startIdx int, endIdx int) (int, error) {
	done := console.EnterInputMode(ctx, fmt.Sprintf("\nEnter a bundle index between [%d - %d]: ", startIdx, endIdx))
	defer done()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		idx, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil || idx < startIdx || idx > endIdx {
			fmt.Printf("please enter a valid index between [%d - %d]: ", startIdx, endIdx)
			continue
		}
		return idx, nil
	}
	return -1, fnerrors.BadInputError("unexpected failure while prompting for the bundle index")
}

type bundleWithInvocationInfo struct {
	bundle         *tasks.Bundle
	invocationInfo *tasks.InvocationInfo
}

func bundlesWithInvocationInfo(ctx context.Context, bundles []*tasks.Bundle) []*bundleWithInvocationInfo {
	var bundlesWithInfo []*bundleWithInvocationInfo
	for _, bundle := range bundles {
		info, err := bundle.ReadInvocationInfo(ctx)
		if err != nil {
			fmt.Fprintf(console.Debug(ctx), "Failed to read invocation info from corrupted bundle: %v", err)
			continue
		}
		bundlesWithInfo = append(bundlesWithInfo, &bundleWithInvocationInfo{bundle, info})
	}
	return bundlesWithInfo
}

func renderBundleTable(ctx context.Context, bundles []*bundleWithInvocationInfo, w io.Writer) error {
	for idx, bundleInfo := range bundles {
		fmt.Fprintf(w, "(%d) %s %s\n", idx+1, bundleInfo.invocationInfo.Command, colors.Faded(humanize.Time(bundleInfo.bundle.Timestamp)))
	}
	return nil
}
