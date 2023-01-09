// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package debug

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func newImageIndexCmd() *cobra.Command {
	var insecure bool

	cmd := &cobra.Command{
		Use:   "get-image-index",
		Short: "Fetches information about an OCI image index.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			for _, arg := range args {
				d, err := fetchImage(ctx, arg, oci.RegistryAccess{InsecureRegistry: insecure})
				if err != nil {
					return err
				}

				if err := printIndex(ctx, d); err != nil {
					return err
				}
			}

			return nil
		}),
	}

	cmd.Flags().BoolVar(&insecure, "insecure", false, "Set to true to access registries over insecure communications.")

	return cmd
}

func fetchImage(ctx context.Context, baseImage string, opts oci.RegistryAccess) (*remote.Descriptor, error) {
	ref, remoteOpts, err := oci.ParseRefAndKeychain(ctx, baseImage, opts)
	if err != nil {
		return nil, err
	}

	desc, err := remote.Get(ref, remoteOpts...)
	if err != nil {
		return nil, err
	}

	return desc, nil
}

func printIndex(ctx context.Context, d *remote.Descriptor) error {
	out := console.Stdout(ctx)

	fmt.Fprintf(out, "index: %s\n", d.Digest.String())
	fmt.Fprintf(out, "mediaType: %v\n", d.MediaType)
	fmt.Fprintf(out, "platform :%v\n", d.Platform)

	index, err := d.ImageIndex()
	if err != nil {
		return err
	}

	im, err := index.IndexManifest()
	if err != nil {
		return err
	}

	for _, m := range im.Manifests {
		fmt.Fprintf(out, "Manifest: %s\n", m.Digest.String())
		fmt.Fprintf(out, " urls: %v\n", m.URLs)
		fmt.Fprintf(out, " mediaType: %v\n", m.MediaType)
		fmt.Fprintf(out, " annotations: %v\n", m.Annotations)
		fmt.Fprintf(out, " platform: %v\n", m.Platform)

		img, err := index.Image(m.Digest)
		if err != nil {
			return err
		}

		if err := printImage(ctx, img); err != nil {
			return fnerrors.BadInputError("failed to print image: %w", err)
		}
	}

	return nil
}
