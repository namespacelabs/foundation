// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package baseimage

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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
		Args:  cobra.ExactArgs(2),
	}

	annotateWithDigest := run.Flags().StringToString("annotate-with-digest", map[string]string{}, "Add an annotation to the base image with the digest of a specified path. Example: nix.store-digest=/nix")

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		sourceImage, err := oci.ParseImageID(args[0])
		if err != nil {
			return err
		}

		registry, err := getNSCRRegistry(ctx)
		if err != nil {
			return err
		}

		targetRef, err := name.NewTag(prependRegistryEp(registry, args[1]), name.StrictValidation)
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

		annotations, err := makeAnnotations(ctx, desc, *annotateWithDigest)
		if err != nil {
			return err
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

		var h v1.Hash
		switch {
		case desc.MediaType.IsIndex():
			index, err := desc.ImageIndex()
			if err != nil {
				return err
			}

			annotated := mutate.Annotations(index, annotations).(v1.ImageIndex)
			if h, err = annotated.Digest(); err != nil {
				return err
			}

			if err = remote.WriteIndex(targetRef, annotated, remoteOpts...); err != nil {
				return err
			}
		case desc.MediaType.IsImage():
			img, err := desc.Image()
			if err != nil {
				return err
			}

			annotated := mutate.Annotations(img, annotations).(v1.Image)
			if h, err = annotated.Digest(); err != nil {
				return err
			}

			if err := remote.Write(targetRef, annotated, remoteOpts...); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported image type: %s", desc.MediaType)
		}

		fmt.Fprintf(console.Info(ctx), "%s: %s@%s\n", colors.Ctx(ctx).Highlight.Apply("✔ Uploaded base image"), targetRef, h)
		console.WriteJSON(console.Info(ctx), "with annotations:", annotations)
		return nil
	})

	return run
}

func makeAnnotations(ctx context.Context, desc *remote.Descriptor, annotateWithDigest map[string]string) (map[string]string, error) {
	img, err := desc.Image()
	if err != nil {
		return nil, err
	}

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
