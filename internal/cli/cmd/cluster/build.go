// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/internal/sdk/buildctl"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/go-ids"
)

var (
	// preferredBuildPlatform is a mapping between supported platforms and preferable build cluster.
	preferredBuildPlatform = map[string]buildPlatform{
		"linux/arm64":  "arm64",
		"linux/arm/v5": "arm64",
		"linux/arm/v6": "arm64",
		"linux/arm/v7": "arm64",
		"linux/arm/v8": "arm64",
	}
)

func NewBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image in a build cluster.",
		Args:  cobra.MaximumNArgs(1),
	}

	dockerFile := cmd.Flags().StringP("file", "f", "", "If set, specifies what Dockerfile to build.")
	push := cmd.Flags().BoolP("push", "p", false, "If specified, pushes the image to the target repository.")
	dockerLoad := cmd.Flags().Bool("load", false, "If specified, load the image to the local docker registry.")
	tags := cmd.Flags().StringSliceP("tag", "t", nil, "Attach the specified tags to the image.")
	platforms := cmd.Flags().StringSlice("platform", []string{}, "Set target platform for build.")
	buildArg := cmd.Flags().StringSlice("build-arg", nil, "Pass build arguments to the build.")
	names := cmd.Flags().StringSliceP("name", "n", nil, "Provide a list of name tags for the image in nscr.io Workspace registry")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, specifiedArgs []string) error {
		if len(*tags) > 0 && len(*names) > 0 {
			return fnerrors.New("usage of both --tag and --name flags is not supported")
		}

		// XXX: having multiple outputs is not supported by buildctl.
		if *push && *dockerLoad {
			return fnerrors.New("usage of both --push and --load flags is not supported")
		}

		if len(*platforms) > 1 && *dockerLoad {
			return fnerrors.New("multi-platform builds require --push, --load is not supported")
		}

		if len(*tags)+len(*names) == 0 && *push {
			return fnerrors.New("--push requires at least one tag or name")
		}

		if len(*platforms) == 0 {
			if *dockerLoad {
				*platforms = []string{platform.FormatPlatform(docker.HostPlatform())}
			} else {
				*platforms = []string{"linux/amd64"}
			}
		}

		buildctlBin, err := buildctl.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return fnerrors.New("failed to download buildctl: %w", err)
		}

		clusterProfiles, err := api.GetProfile(ctx, api.Endpoint)
		if err != nil {
			return err
		}

		contextDir := "."
		if len(specifiedArgs) > 0 {
			contextDir = specifiedArgs[0]
		}

		if len(*names) > 0 {
			// Either tags or names slice is set, but not both.
			// So, append all names in tags slice
			resp, err := api.GetImageRegistry(ctx, api.Endpoint)
			if err != nil {
				return fmt.Errorf("Could not fetch nscr.io repository: %w", err)
			}

			if resp.NSCR == nil {
				return fmt.Errorf("Could not fetch nscr.io repository")
			}

			for _, name := range *names {
				*tags = append(*tags, fmt.Sprintf("%s/%s/%s", resp.NSCR.EndpointAddress, resp.NSCR.Repository, name))
			}
		}

		parsedTags := make([]name.Tag, len(*tags))
		for k, tag := range *tags {
			parsed, err := name.NewTag(tag)
			if err != nil {
				return fmt.Errorf("invalid tag %s: %w", tag, err)
			}
			parsedTags[k] = parsed
		}

		// Allocate a list ahead of time to allow for concurrent access.
		completeFuncs := make([]func() error, len(*platforms))

		builders := map[buildPlatform]*buildProxyWithRegistry{}
		for _, p := range *platforms {
			resolved := determineBuildClusterPlatform(clusterProfiles.ClusterPlatform, p)
			if _, ok := builders[resolved]; !ok {
				buildProxy, err := runBuildProxyWithRegistry(ctx, resolved, len(*names) > 0)
				if err != nil {
					return err
				}
				defer buildProxy.Cleanup()

				builders[resolved] = buildProxy
			}
		}

		eg := executor.New(ctx, "nsc/build")

		images := make([]oci.RawImageWithPlatform, len(*platforms))
		for k, p := range *platforms {
			platformSpec, err := platform.ParsePlatform(p)
			if err != nil {
				return err
			}

			var imageNames []string

			// When performing a multi-platform build, we only need a single
			// remote reference to point an index at.
			if len(*platforms) > 1 && len(parsedTags) > 0 {
				imageNames = append(imageNames, fmt.Sprintf("%s:%s-%s", parsedTags[0].Repository.Name(), strings.ReplaceAll(platform.FormatPlatform(platformSpec), "/", "-"), ids.NewRandomBase32ID(4)))
			} else {
				for _, parsed := range parsedTags {
					imageNames = append(imageNames, parsed.Name())
				}
			}

			args := []string{
				"build",
				"--frontend=dockerfile.v0",
				"--local", "context=" + contextDir,
				"--local", "dockerfile=" + contextDir,
				"--opt", "platform=" + p,
			}

			if *dockerFile != "" {
				args = append(args, "--opt", "filename="+*dockerFile)
			}

			for _, arg := range *buildArg {
				args = append(args, "--opt", "build-arg:"+arg)
			}

			k := k // Capture k.
			switch {
			case *push:
				// buildctl parses output as csv; need to quote to pass commas to `name`.
				args = append(args, "--output", fmt.Sprintf("type=image,push=true,%q", "name="+strings.Join(imageNames, ",")))

				completeFuncs[k] = func() error {
					fmt.Fprintf(console.Stdout(ctx), "Pushed for %s:\n", p)
					for _, imageName := range imageNames {
						fmt.Fprintf(console.Stdout(ctx), "  %s\n", imageName)
					}

					ref, remoteOpts, err := oci.ParseRefAndKeychain(ctx, imageNames[0], oci.RegistryAccess{Keychain: api.DefaultKeychain})
					if err != nil {
						return err
					}

					image, err := remote.Image(ref, remoteOpts...)
					if err != nil {
						return fnerrors.InvocationError("registry", "failed to fetch image: %w", err)
					}

					images[k] = oci.RawImageWithPlatform{
						Image:    image,
						Platform: platformSpec,
					}

					return nil
				}

			case *dockerLoad:
				// Load to local Docker registry
				f, err := os.CreateTemp("", "docker-image-nsc")
				if err != nil {
					return err
				}

				// We don't actually need it.
				f.Close()

				if len(imageNames) > 0 {
					// buildctl parses output as csv; need to quote to pass commas to `name`.
					args = append(args, "--output", fmt.Sprintf("type=docker,dest=%s,%q", f.Name(), "name="+strings.Join(imageNames, ",")))
				} else {
					args = append(args, "--output", fmt.Sprintf("type=docker,dest=%s", f.Name()))
				}

				completeFuncs[k] = func() error {
					defer os.Remove(f.Name())

					t := time.Now()
					dockerLoad := exec.CommandContext(ctx, "docker", "load", "-i", f.Name())
					if err := localexec.RunInteractive(ctx, dockerLoad); err != nil {
						return err
					}
					fmt.Fprintf(console.Stdout(ctx), "Took %v to upload the image to Docker.\n", time.Since(t))
					return nil
				}
			}

			eg.Go(func(ctx context.Context) error {
				return runBuildctl(ctx, buildctlBin, builders[determineBuildClusterPlatform(clusterProfiles.ClusterPlatform, p)], args...)
			})
		}

		if err := eg.Wait(); err != nil {
			return err
		}

		for _, fn := range completeFuncs {
			if fn != nil {
				if err := fn(); err != nil {
					return err
				}
			}
		}

		if len(images) > 1 && *push {
			index, err := oci.RawMakeIndex(images...)
			if err != nil {
				return err
			}

			fmt.Fprint(console.Stdout(ctx), "Multi-platform image:\n\n")

			for _, parsed := range parsedTags {
				if _, err := index.Push(ctx, oci.RepositoryWithAccess{
					Repository: parsed.Name(),
					RegistryAccess: oci.RegistryAccess{
						Keychain: api.DefaultKeychain,
					},
				}, false); err != nil {
					return err
				}

				fmt.Fprintf(console.Stdout(ctx), "  %s\n", parsed.Name())
			}
		}

		if !*push {
			// On push, we already report what was built. Add a hint for other builds, too.
			fmt.Fprintf(console.Stdout(ctx), "\nBuilt %d images (platforms %s).\n", len(images), strings.Join(*platforms, ","))
		}

		return nil
	})

	return cmd
}

func determineBuildClusterPlatform(allowedClusterPlatforms []string, platform string) buildPlatform {
	// If requested platform is arm64 and "arm64" is allowed.
	if preferredBuildPlatform[platform] == "arm64" && slices.Contains(allowedClusterPlatforms, "arm64") {
		return "arm64"
	}

	return "amd64"
}

func imageNameWithPlatform(imageName string, platformSpec specs.Platform) string {
	return fmt.Sprintf("%s-%s", imageName, platformSpec.Architecture)
}

func normalizeReference(ref string) (reference.Named, error) {
	namedRef, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return nil, fnerrors.New("failed to parse image reference: %w", err)
	}
	if _, isDigested := namedRef.(reference.Canonical); !isDigested {
		return reference.TagNameOnly(namedRef), nil
	}
	return namedRef, nil
}

func buildClusterOpts(platform buildPlatform) api.EnsureBuildClusterOpts {
	var opts api.EnsureBuildClusterOpts
	if platform == "arm64" {
		opts.Features = []string{"EXP_ARM64_CLUSTER"}
	}

	return opts
}
