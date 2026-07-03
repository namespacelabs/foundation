// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package baseimage

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newUploadBaseImageCmd() *cobra.Command {
	run := &cobra.Command{
		Use:   "upload [source-image] [target-tag]",
		Short: "Converts an existing image into a base image and uploads it to nscr.io",
		Args:  cobra.RangeArgs(1, 2),
	}

	annotateWithDigest := run.Flags().StringToString("annotate-with-digest", map[string]string{}, "Add an annotation to the base image with the digest of a specified path. Example: nix.store-digest=/nix")
	dryRun := run.Flags().Bool("dry-run", false, "Pull source image and calculate annotations without pushing to nscr.io")
	fromFile := run.Flags().String("from-file", "", "Load the source image from a local tar file (e.g. output of 'podman save' or 'docker save') instead of pulling from a remote registry")

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		// When --from-file is set: upload [target-tag]
		// Otherwise:              upload [source-image] [target-tag]
		if *fromFile == "" && len(args) < 2 {
			return fmt.Errorf("source-image argument is required when --from-file is not set")
		}

		targetArg := args[len(args)-1]

		registry, err := getNSCRRegistry(ctx)
		if err != nil {
			return err
		}

		targetRef, err := name.NewTag(prependRegistryEp(registry, targetArg), name.StrictValidation)
		if err != nil {
			return err
		}

		var img v1.Image

		if *fromFile != "" {
			fmt.Fprintf(console.Info(ctx), "Loading image from file: %s\n", *fromFile)
			img, err = imageFromFile(*fromFile)
			if err != nil {
				return fmt.Errorf("failed to load image from %s: %w", *fromFile, err)
			}
		} else {
			sourceImage, err := oci.ParseImageID(args[0])
			if err != nil {
				return err
			}

			fmt.Fprintf(console.Info(ctx), "Pulling image: %s\n", sourceImage)

			desc, err := oci.FetchRemoteDescriptor(
				ctx,
				sourceImage.RepoAndDigest(),
				oci.RegistryAccess{Keychain: api.DefaultKeychainWithFallback},
			)
			if err != nil {
				return fmt.Errorf("failed to fetch %s: %w", sourceImage, err)
			}

			img, err = desc.Image()
			if err != nil {
				return fmt.Errorf("failed to load image from descriptor: %w", err)
			}
		}

		annotations, err := makeAnnotations(ctx, img, *annotateWithDigest)
		if err != nil {
			return err
		}
		console.WriteJSON(console.Info(ctx), "Image annotations:", annotations)

		if *dryRun {
			fmt.Fprintf(console.Info(ctx), "Dry run, not pushing image: %s\n", targetRef)
			return nil
		}

		fmt.Fprintf(console.Info(ctx), "Pushing image: %s\n", targetRef)

		remoteOpts, err := oci.RemoteOptsWithAuth(
			ctx,
			oci.RegistryAccess{Keychain: api.DefaultKeychain},
			true,
		)
		if err != nil {
			return err
		}

		annotated := mutate.Annotations(img, annotations).(v1.Image)
		h, err := annotated.Digest()
		if err != nil {
			return err
		}
		if err := remote.Write(targetRef, annotated, remoteOpts...); err != nil {
			return err
		}

		fmt.Fprintf(console.Info(ctx), "%s: %s@%s\n", colors.Ctx(ctx).Highlight.Apply("✔ Uploaded base image"), targetRef, h)
		return nil
	})

	return run
}

// imageFromFile loads an OCI image from a local tar file (docker/podman save format).
// The opener re-opens the file on each call so layers are read from disk on demand
// rather than buffered into memory.
func imageFromFile(path string) (v1.Image, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return tarball.Image(func() (io.ReadCloser, error) {
		return os.Open(path)
	}, nil)
}

func makeAnnotations(ctx context.Context, img v1.Image, annotateWithDigest map[string]string) (map[string]string, error) {
	annotations := map[string]string{
		"nsc.base-image": "v1",
	}

	for annotationName, path := range annotateWithDigest {
		fmt.Fprintf(console.Info(ctx), "Calculating digest of path %s\n", path)
		digest, err := oci.HashPathInImage(img, path, oci.HashPathOpts{IncludeLinkNames: true})
		if err != nil {
			return nil, fmt.Errorf("failed to calculate digest of path %s: %v", path, err)
		}

		fmt.Fprintf(console.Info(ctx), "Adding annotation %s with digest of path %s: %s\n", annotationName, path, digest)
		annotations[annotationName] = digest.String()
	}

	return annotations, nil
}

func getNSCRRegistry(ctx context.Context) (string, error) {
	resp, err := api.GetImageRegistry(ctx, api.Methods)
	if err != nil {
		return "", fmt.Errorf("could not fetch nscr.io repository: %w", err)
	}

	if resp.NSCR == nil {
		return "", fmt.Errorf("could not fetch nscr.io repository")
	}

	return fmt.Sprintf("%s/%s/", resp.NSCR.EndpointAddress, resp.NSCR.Repository), nil
}

func prependRegistryEp(registryEp string, tag string) string {
	if strings.HasPrefix(tag, registryEp) {
		return tag
	}

	return registryEp + tag
}
