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
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

var (
	// preferredBuildPlatform is a mapping between supported platforms and preferable build cluster.
	preferredBuildPlatform = map[string]api.BuildPlatform{
		"linux/arm64":  "arm64",
		"linux/arm/v5": "arm64",
		"linux/arm/v6": "arm64",
		"linux/arm/v7": "arm64",
		"linux/arm/v8": "arm64",
	}
)

func NewBaseImageBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-base-image",
		Short: "Test the build of a base image.",
		Args:  cobra.NoArgs,
	}

	dockerFile := cmd.Flags().StringP("file", "f", "", "Specifies what Dockerfile to build.")

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

		buildArgs := map[string]string{
			"NAMESPACE_BASE_IMAGE_REF": "foobar",
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

		if _, err := cluster.StartBuilds(ctx, fragments, cluster.WireBuilder); err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "\nBuilt %d %s (platforms %s).\n", len(fragments), plural(len(fragments), "image", "images"), strings.Join(platforms, ","))

		return nil
	})

	return cmd
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}

	return plural
}
