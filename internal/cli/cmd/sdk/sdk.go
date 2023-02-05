// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package sdk

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/morikuni/aec"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/sdk/deno"
	"namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/internal/sdk/grpcurl"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/internal/sdk/nodejs"
	"namespacelabs.dev/foundation/std/tasks"
)

func NewSdkCmd() *cobra.Command {
	sdks := []string{"go", "nodejs", "k3d", "kubectl", "grpcurl", "deno"}

	goSdkVersion := "1.20"
	nodejsVersion := "18"

	cmd := &cobra.Command{
		Use:   "sdk",
		Short: "SDK related operations (e.g. download, shell).",
	}

	selectedSdkList := func() []sdk {
		return sdkList(sdks, goSdkVersion, nodejsVersion)
	}

	cmd.PersistentFlags().StringVar(&goSdkVersion, "go_version", goSdkVersion, "Go version.")
	cmd.PersistentFlags().StringVar(&nodejsVersion, "nodejs_version", nodejsVersion, "NodeJS version.")
	cmd.PersistentFlags().StringArrayVar(&sdks, "sdks", sdks, "The SDKs we download.")

	cmd.AddCommand(newSdkShellCmd(selectedSdkList))
	cmd.AddCommand(newSdkDownloadCmd(selectedSdkList))
	cmd.AddCommand(newSdkVerifyCmd(selectedSdkList))
	cmd.AddCommand(newGoCmd(goSdkVersion))
	cmd.AddCommand(newGcloudCmd())

	return cmd
}

type sdk struct {
	name     string
	make     func(context.Context, string, *compute.In, specs.Platform) (*compute.In, error)
	makePath func(compute.Resolved, string) (string, error)
}

func sdkList(sdks []string, goVersion, nodejsVersion string) []sdk {
	var available = []sdk{
		{
			name: "go",
			make: func(ctx context.Context, key string, in *compute.In, p specs.Platform) (*compute.In, error) {
				sdk, err := golang.SDK(goVersion, p)
				if err != nil {
					return nil, err
				}
				return in.Computable(key, sdk), nil
			},
			makePath: func(deps compute.Resolved, key string) (string, error) {
				sdk, _ := compute.GetDepWithType[golang.LocalSDK](deps, key)
				return filepath.Dir(sdk.Value.Binary), nil
			},
		},
		{
			name: "nodejs",
			make: func(ctx context.Context, key string, in *compute.In, p specs.Platform) (*compute.In, error) {
				sdk, err := nodejs.SDK(nodejsVersion, p)
				if err != nil {
					return nil, err
				}
				return in.Computable(key, sdk), nil
			},
			makePath: func(deps compute.Resolved, key string) (string, error) {
				sdk, _ := compute.GetDepWithType[nodejs.LocalSDK](deps, key)
				return filepath.Dir(sdk.Value.Binary), nil
			},
		},
		simpleFileSDK("k3d", k3d.SDK),
		simpleFileSDK("kubectl", kubectl.SDK),
		simpleFileSDK("grpcurl", grpcurl.SDK),
		simpleFileSDK("deno", deno.SDK),
	}

	var ret []sdk
	for _, sdk := range available {
		if slices.Contains(sdks, sdk.name) {
			ret = append(ret, sdk)
		}
	}
	return ret
}

func simpleFileSDK[V ~string](name string, makeComputable func(context.Context, specs.Platform) (compute.Computable[V], error)) sdk {
	return sdk{
		name: name,
		make: func(ctx context.Context, key string, in *compute.In, p specs.Platform) (*compute.In, error) {
			sdk, err := makeComputable(ctx, p)
			if err != nil {
				return nil, err
			}
			return in.Computable(key, sdk), nil
		},
		makePath: func(deps compute.Resolved, key string) (string, error) {
			sdk, _ := compute.GetDepWithType[V](deps, key)
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
				return fnerrors.New("No $SHELL defined.")
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

			var shellArgs, shellEnv []string
			switch filepath.Base(shell) {
			case "bash":
				prompt := fmt.Sprintf("%s %s %s \\w$ ",
					aec.LightBlueF.Apply("(ns)"),
					aec.LightGreenF.Apply("\\u"),
					aec.LightBlackF.Apply("\\h"))

				shellArgs = []string{"--login", "--noprofile"}
				shellEnv = []string{"PS1=" + prompt}
			case "zsh":
				prompt := fmt.Sprintf("%s %s %s %%~$ ",
					aec.LightBlueF.Apply("(ns)"),
					aec.LightGreenF.Apply("%n"),
					aec.LightBlackF.Apply("%m"))

				shellArgs = []string{"--no-rcs"}
				shellEnv = []string{"PROMPT=" + prompt}
			}

			cmd := exec.CommandContext(ctx, shell, shellArgs...)
			cmd.Env = append(os.Environ(), "PATH="+updatedPath)
			cmd.Env = append(cmd.Env, shellEnv...)
			return localexec.RunInteractive(ctx, cmd)
		}),
	}

	return cmd
}

func newSdkDownloadCmd(selectedSdkList func() []sdk) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Downloads a predefined set of SDKs.",
		Args:  cobra.NoArgs,
	}

	outputPath := cmd.Flags().String("output_to", "", "If specified, write the paths of the downloaded SDKs to this path.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		downloads, err := makeDownloads(ctx, selectedSdkList())
		if err != nil {
			return err
		}

		r, err := compute.Get(ctx, compute.Collect(tasks.Action("sdk.download-all"), downloads...))
		if err != nil {
			return err
		}

		if *outputPath != "" {
			var out bytes.Buffer
			for k, res := range r.Value {
				if k > 0 {
					fmt.Fprintln(&out)
				}
				fmt.Fprint(&out, res.Value)
			}

			if err := os.WriteFile(*outputPath, out.Bytes(), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *outputPath, err)
			}
		}

		fmt.Fprintf(console.Stdout(ctx), "Downloaded %d SDKs.\n", len(r.Value))

		return nil
	})

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

			platforms := []specs.Platform{
				{Architecture: "amd64", OS: "linux"},
				{Architecture: "arm64", OS: "linux"},
				{Architecture: "amd64", OS: "darwin"},
				{Architecture: "arm64", OS: "darwin"},
			}

			in := compute.Inputs()
			for _, sdk := range selectedSdkList() {
				for _, p := range platforms {
					newIn, err := sdk.make(ctx, fmt.Sprintf("%s-%s", sdk.name, platform.FormatPlatform(p)), in, p)
					if err != nil {
						fmt.Fprintf(console.Warnings(ctx), "Skipped %q (%s): %v\n", sdk.name, platform.FormatPlatform(p), err)
					} else {
						in = newIn
					}
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
		c, err := sdk.make(ctx, sdk.name, compute.Inputs(), host.HostPlatform())
		if err != nil {
			return nil, err
		}

		sdk := sdk // Close sdk.

		downloads = append(downloads, compute.Map(
			tasks.Action("sdk.download"),
			c, compute.Output{},
			func(ctx context.Context, deps compute.Resolved) (string, error) {
				return sdk.makePath(deps, sdk.name)
			}))
	}
	return downloads, nil
}
