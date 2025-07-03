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
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/parsing/platform"
)

func NewBaseImageBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-base-image",
		Short: "Test the build of a base image.",
		Args:  cobra.NoArgs,
	}

	dockerFile := cmd.Flags().StringP("file", "f", "", "Specifies what Dockerfile to build.")
	osLabel := cmd.Flags().StringP("os-label", "l", "ubuntu-22.04", "Specifies the OS version of the base image.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, specifiedArgs []string) error {
		if *dockerFile == "" {
			return fmt.Errorf("-f is required.")
		}

		dockerFileContents, err := os.ReadFile(*dockerFile)
		if err != nil {
			return err
		}

		platforms := []string{
			"linux/amd64",
			"linux/arm64",
		}

		imgRef, err := getBaseImageRef(ctx, *osLabel)
		if err != nil {
			return err
		}

		buildArgs := map[string]string{
			"NAMESPACE_BASE_IMAGE_REF": imgRef,
		}

		var fragments []cluster.BuildFragment
		for _, p := range platforms {
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

		if _, err := cluster.StartBuilds(ctx, fragments, cluster.MakeWireBuilder(false, "")); err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "\nBuilt %d %s (platforms %s).\n", len(fragments), plural(len(fragments), "image", "images"), strings.Join(platforms, ","))

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
