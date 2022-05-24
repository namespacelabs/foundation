// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/morikuni/aec"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/internal/sdk/grpcurl"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/internal/sdk/octant"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewSdkCmd() *cobra.Command {
	sdks := []string{"go", "k3d", "kubectl", "grpcurl", "octant"}

	goSdkVersion := "1.18"

	cmd := &cobra.Command{
		Use:   "sdk",
		Short: "SDK related operations (e.g. download, shell).",
	}

	selectedSdkList := func() []sdk {
		return sdkList(sdks, goSdkVersion)
	}

	cmd.PersistentFlags().StringVar(&goSdkVersion, "go_version", goSdkVersion, "Go version.")
	cmd.PersistentFlags().StringArrayVar(&sdks, "sdks", sdks, "The SDKs we download.")

	cmd.AddCommand(newSdkShellCmd(selectedSdkList))
	cmd.AddCommand(newSdkDownloadCmd(selectedSdkList))
	cmd.AddCommand(newSdkVerifyCmd(selectedSdkList))

	return cmd
}

type sdk struct {
	name     string
	make     func(context.Context, *compute.In, bool) (*compute.In, error)
	makePath func(deps compute.Resolved) (string, error)
}

func sdkList(sdks []string, goVersion string) []sdk {
	var available = []sdk{
		{
			name: "go",
			make: func(ctx context.Context, in *compute.In, verify bool) (*compute.In, error) {
				sdk, err := golang.SDK(goVersion, golang.HostPlatform())
				if err != nil {
					return nil, err
				}
				return in.Computable("sdk", sdk), nil
			},
			makePath: func(deps compute.Resolved) (string, error) {
				goSdk, _ := compute.GetDepWithType[golang.LocalSDK](deps, "sdk")
				return filepath.Join(goSdk.Value.Path, "go/bin"), nil
			},
		},
		simpleFileSDK("k3d", k3d.SDK, k3d.AllDownloads),
		simpleFileSDK("kubectl", kubectl.SDK, kubectl.AllDownloads),
		simpleFileSDK("octant", octant.SDK, octant.AllDownloads),
		simpleFileSDK("grpcurl", grpcurl.SDK, grpcurl.AllDownloads),
	}

	var ret []sdk
	for _, sdk := range available {
		if slices.Contains(sdks, sdk.name) {
			ret = append(ret, sdk)
		}
	}
	return ret
}

func simpleFileSDK[V ~string](name string, makeComputable func(context.Context) (compute.Computable[V], error),
	allDownloads func() []compute.Computable[bytestream.ByteStream]) sdk {
	return sdk{
		name: name,
		make: func(ctx context.Context, in *compute.In, verify bool) (*compute.In, error) {
			if verify {
				for k, download := range allDownloads() {
					in = in.Computable(fmt.Sprintf("%s:%d", name, k), download)
				}
				return in, nil
			}

			sdk, err := makeComputable(ctx)
			if err != nil {
				return nil, err
			}
			return in.Computable(name, sdk), nil
		},
		makePath: func(deps compute.Resolved) (string, error) {
			sdk, _ := compute.GetDepWithType[V](deps, name)
			return filepath.Dir(string(sdk.Value)), nil
		},
	}
}

func newSdkShellCmd(selectedSdkList func() []sdk) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Starts a shell with every SDK in the PATH.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			_, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			shell := os.Getenv("SHELL")
			if shell == "" {
				return fnerrors.UserError(nil, "No $SHELL defined.")
			}

			updatedPath := os.Getenv("PATH")

			downloads, err := makeDownloads(ctx, selectedSdkList())
			if err != nil {
				return err
			}

			results, err := compute.GetValue(ctx, compute.Collect(tasks.Action("download-all"), downloads...))
			if err != nil {
				return err
			}

			for _, r := range results {
				updatedPath = prependPath(updatedPath, r.Value)
			}

			done := console.EnterInputMode(ctx)
			defer done()

			var shellArgs, shellEnv []string
			switch filepath.Base(shell) {
			case "bash":
				prompt := fmt.Sprintf("%s %s %s \\w$ ",
					aec.LightBlueF.Apply("(fn)"),
					aec.LightGreenF.Apply("\\u"),
					aec.LightBlackF.Apply("\\h"))

				shellArgs = []string{"--login", "--noprofile"}
				shellEnv = []string{"PS1=" + prompt}
			case "zsh":
				prompt := fmt.Sprintf("%s %s %s %%~$ ",
					aec.LightBlueF.Apply("(fn)"),
					aec.LightGreenF.Apply("%n"),
					aec.LightBlackF.Apply("%m"))

				shellArgs = []string{"--no-rcs"}
				shellEnv = []string{"PROMPT=" + prompt}
			}

			cmd := exec.CommandContext(ctx, shell, shellArgs...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = append(os.Environ(), "PATH="+updatedPath)
			cmd.Env = append(cmd.Env, shellEnv...)
			return cmd.Run()
		}),
	}

	return cmd
}

func newSdkDownloadCmd(selectedSdkList func() []sdk) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Downloads a predefined set of SDKs.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			_, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			downloads, err := makeDownloads(ctx, selectedSdkList())
			if err != nil {
				return err
			}

			_, err = compute.Get(ctx, compute.Collect(tasks.Action("sdk.download-all"), downloads...))
			return err
		}),
	}

	return cmd
}

func newSdkVerifyCmd(selectedSdkList func() []sdk) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "verify-digest",
		Short:  "Downloads all of the SDKs known URLs to verify their digests (for development).",
		Hidden: true,
		Args:   cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			_, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			in := compute.Inputs()
			for _, sdk := range selectedSdkList() {
				var err error
				in, err = sdk.make(ctx, in, true)
				if err != nil {
					return err
				}
			}

			_, err = compute.Get(ctx, compute.Map(tasks.Action("sdk.verify-digest"), in, compute.Output{},
				func(ctx context.Context, r compute.Resolved) (int, error) {
					// Just want to make sure we can download them.
					return 0, nil
				}))

			return err
		}),
	}

	return cmd
}

func prependPath(existing, additional string) string {
	if existing == "" {
		return additional
	}
	return additional + ":" + existing
}

func makeDownloads(ctx context.Context, sdks []sdk) ([]compute.Computable[string], error) {
	var downloads []compute.Computable[string]
	for _, sdk := range sdks {
		c, err := sdk.make(ctx, compute.Inputs(), false)
		if err != nil {
			return nil, err
		}

		sdk := sdk // Close sdk.

		downloads = append(downloads, compute.Map(
			tasks.Action("sdk.download"),
			c, compute.Output{},
			func(ctx context.Context, deps compute.Resolved) (string, error) {
				return sdk.makePath(deps)
			}))
	}
	return downloads, nil
}
