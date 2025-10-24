// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package github

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/parsing/platform"
)

func NewBaseImageBuildCmd(use string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: "Test the build of a base image.",
		Args:  cobra.NoArgs,
	}

	dockerFile := cmd.Flags().StringP("file", "f", "", "Specifies what Dockerfile to build.")
	osLabel := cmd.Flags().StringP("os-label", "l", "ubuntu-22.04", "Specifies the OS version of the base image.")
	platforms := cmd.Flags().StringSliceP("platform", "p", []string{"linux/amd64", "linux/arm64"}, "Which platforms to build for (linux/amd64 or linux/arm64)")

	useServerSideProxy := cmd.Flags().Bool("use_server_side_proxy", true, "If set, client is setup to use transparent mTLS server-side proxy instead of websockets.")
	_ = cmd.Flags().MarkHidden("use_server_side_proxy")
	waitUntilReady := cmd.Flags().Bool("wait_until_ready", true, "If set, wait for build cluster readiness before dialing build connections.")
	_ = cmd.Flags().MarkHidden("wait_until_ready")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, specifiedArgs []string) error {
		if *dockerFile == "" {
			return fmt.Errorf("-f is required.")
		}

		dockerFileContents, err := os.ReadFile(*dockerFile)
		if err != nil {
			return err
		}

		imgRef, err := getBaseImageRef(ctx, *osLabel)
		if err != nil {
			return err
		}

		buildArgs := map[string]string{
			"NAMESPACE_BASE_IMAGE_REF": imgRef,
		}

		supportedPlatforms := []string{
			"linux/amd64",
			"linux/arm64",
		}

		var fragments []cluster.BuildFragment
		for _, p := range *platforms {
			if !slices.Contains(supportedPlatforms, p) {
				return fmt.Errorf("platform %s not supported", p)
			}

			platformSpec, err := platform.ParsePlatform(p)
			if err != nil {
				return err
			}

			bf := cluster.BuildFragment{
				DockerfileContents: dockerFileContents,
				Platform:           platformSpec,
				BuildArgs:          buildArgs,
			}

			fragments = append(fragments, bf)
		}

		if _, err := cluster.StartBuilds(ctx, fragments, cluster.MakeWireBuilder(*useServerSideProxy, "", *waitUntilReady)); err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "\nBuilt %d %s (platforms %s).\n", len(fragments), plural(len(fragments), "image", "images"), strings.Join(*platforms, ","))

		return nil
	})

	return cmd
}

func getBaseImageRef(ctx context.Context, osLabel string) (string, error) {
	images, err := fnapi.ListBaseImages(ctx, osLabel)
	if err != nil {
		return "", err
	}

	if len(images.Images) == 0 {
		return "", fmt.Errorf("no base image found for os label %q", osLabel)
	}

	if len(images.Images) > 1 {
		return "", fmt.Errorf("os %q label maps to more than one base image", osLabel)
	}

	return images.Images[0].ImageRef, nil
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}

	return plural
}
