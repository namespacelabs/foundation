// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package macos

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/dustin/go-humanize"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newPackageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package [path]",
		Short: "Creates a macOS package from a directory and uploads it to nscr.io",
		Args:  cobra.ExactArgs(1),
	}

	imageName := cmd.Flags().StringP("name", "n", "", "Name tag for the image in nscr.io workspace registry")
	_ = cmd.MarkFlagRequired("name")
	output := cmd.Flags().StringP("output", "o", "plain", "Output format: plain, json")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		contentPath := args[0]

		info, err := os.Stat(contentPath)
		if err != nil {
			return fnerrors.Newf("failed to access path %q: %w", contentPath, err)
		}

		if !info.IsDir() {
			return fnerrors.Newf("path %q is not a directory", contentPath)
		}

		var tarBytes bytes.Buffer
		w := tar.NewWriter(&tarBytes)
		if err := w.AddFS(os.DirFS(contentPath)); err != nil {
			return fnerrors.Newf("failed to create tar: %w", err)
		}
		if err := w.Close(); err != nil {
			return fnerrors.Newf("failed to finalize tar: %w", err)
		}

		newImage, err := mutate.AppendLayers(empty.Image, stream.NewLayer(io.NopCloser(bytes.NewReader(tarBytes.Bytes()))))
		if err != nil {
			return fnerrors.Newf("failed to produce image: %w", err)
		}

		resp, err := api.GetImageRegistry(ctx, api.Methods)
		if err != nil {
			return fnerrors.Newf("could not fetch nscr.io repository: %w", err)
		}

		if resp.NSCR == nil {
			return fnerrors.Newf("could not fetch nscr.io repository")
		}

		fullTag := fmt.Sprintf("%s/%s/%s", resp.NSCR.EndpointAddress, resp.NSCR.Repository, *imageName)
		parsed, err := name.NewTag(fullTag)
		if err != nil {
			return fnerrors.Newf("failed to parse image ref %q: %w", fullTag, err)
		}

		fmt.Fprintf(console.Stderr(ctx), "Pushing image: %s ...\n", parsed.Name())

		remoteOpts, err := oci.RemoteOptsWithAuth(ctx, oci.RegistryAccess{Keychain: api.DefaultKeychain}, true)
		if err != nil {
			return fnerrors.Newf("failed to create remote options: %w", err)
		}

		if err := remote.Write(parsed, newImage, remoteOpts...); err != nil {
			return fnerrors.Newf("failed to push image: %w", err)
		}

		digest, err := newImage.Digest()
		if err != nil {
			return fnerrors.Newf("failed to compute digest: %w", err)
		}

		layers, err := newImage.Layers()
		if err != nil {
			return fnerrors.Newf("failed to get layers: %w", err)
		}

		var totalSize int64
		for _, layer := range layers {
			size, err := layer.Size()
			if err != nil {
				return fnerrors.Newf("failed to get layer size: %w", err)
			}
			totalSize += size
		}

		imageRef := parsed.Digest(digest.String()).String()

		switch *output {
		case "json":
			result := PackageResult{
				ImageRef: imageRef,
				Size:     totalSize,
			}
			enc := json.NewEncoder(console.Stdout(ctx))
			enc.SetIndent("", "  ")
			return enc.Encode(result)

		case "plain":
			fmt.Fprintf(console.Stdout(ctx), "\nPushed: %s\n", imageRef)
			fmt.Fprintf(console.Stdout(ctx), "Size: %s\n", humanize.IBytes(uint64(totalSize)))
			return nil

		default:
			return fnerrors.BadInputError("invalid output format: %s", *output)
		}
	})

	return cmd
}

type PackageResult struct {
	ImageRef string `json:"image_ref"`
	Size     int64  `json:"size"`
}
