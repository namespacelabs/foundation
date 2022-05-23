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
	"github.com/morikuni/aec"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
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
		Short: "Lists previous command invocations.",
		Args:  cobra.NoArgs,

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
		Args:  cobra.NoArgs,

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

			// Create a temporary age file that we encrypt and whose contents will be uploaded.
			file, err := dirs.CreateUserTemp("action-bundles", "actions-*.tar.gz.age")
			if err != nil {
				return fnerrors.InternalError("failed to create the temporary `age` file: %w", err)
			}
			encBundlePath := file.Name()
			defer os.Remove(encBundlePath)

			bundleInfo := validBundles[idx-1]
			if err := bundleInfo.bundle.EncryptTo(ctx, file); err != nil {
				return err
			}

			// Please note that we need to close and re-open the encrypted bundle to succesfully post the contents
			// to the bundle service. The body is empty if we directly pass the open file handler above.
			if err := file.Close(); err != nil {
				return fnerrors.InternalError("failed to close temporary encrypted bundle %s: %w", encBundlePath, err)
			}
			bundleContents, err := os.OpenFile(encBundlePath, os.O_RDONLY, 0600)
			if err != nil {
				return fnerrors.InternalError("failed to open the flushed encrypted bundle %s: %w", encBundlePath, err)
			}

			invokedCmd := bundleInfo.invocationInfo.Command
			return fnapi.UploadBundle(ctx, bundleContents, func(res *fnapi.UploadBundleResponse) error {
				w := console.Stderr(ctx)
				fmt.Fprintf(w, "Uploaded artifacts for %s successfully with fingerprint: %s.\n", aec.MagentaF.Apply(invokedCmd), aec.BlueF.Apply(aec.Bold.Apply(res.BundleId)))
				fmt.Fprintln(w)
				fmt.Fprintf(w, "Please file a bug at https://github.com/namespacelabs/foundation/issues with the command %q and fingerprint %q.\n", invokedCmd, res.BundleId)
				return nil
			})
		}),
	}

	download := &cobra.Command{
		Use:   "download",
		Short: "Downloads an encrypted command bundle from foundation.",
		Args:  cobra.ExactArgs(1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			bundleId := args[0]
			return fnapi.DownloadBundle(ctx, bundleId, func(body io.ReadCloser) error {
				file, err := dirs.CreateUserTemp("action-bundles", "actions-*.tar.gz.age")
				if err != nil {
					return fnerrors.InternalError("failed to create the temporary `age` file to download to: %w\n", err)
				}
				encPath := file.Name()
				defer file.Close()

				if _, err := io.Copy(file, body); err != nil {
					return fnerrors.InternalError("failed to copy downloaded bundle contents to %s: %w\n", encPath, err)
				}
				fmt.Fprintf(console.Stderr(ctx), "\nSuccessfully downloaded encrypted bundle for fingerprint %s to %s\n", aec.BlueF.Apply(aec.Bold.Apply(bundleId)), encPath)
				return nil
			})
		}),
	}

	cmd.AddCommand(list)
	cmd.AddCommand(upload)
	cmd.AddCommand(download)

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
