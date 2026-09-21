// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package windows

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newPackageCmd() *cobra.Command {
	var output, imageName string

	cmd := fncobra.Cmd(&cobra.Command{
		Use:   "package [path]",
		Short: "Creates a Windows package from a directory and uploads it to nscr.io",
		Args:  cobra.ExactArgs(1),
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVarP(&output, "output", "o", "plain", "Output format: plain, json")
		flags.StringVarP(&imageName, "name", "n", "", "Name tag for the image in nscr.io workspace registry")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		result, err := packageAndPush(ctx, args[0], imageName)
		if err != nil {
			return err
		}

		switch output {
		case "json":
			enc := json.NewEncoder(console.Stdout(ctx))
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		case "plain":
			fmt.Fprintf(console.Stdout(ctx), "\nPushed: %s\n", result.ImageRef)
			fmt.Fprintf(console.Stdout(ctx), "Size: %s\n", humanize.IBytes(uint64(result.Size)))
			return nil
		default:
			return fnerrors.BadInputError("invalid output format: %s", output)
		}
	})
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

type PackageResult struct {
	ImageRef string `json:"image_ref"`
	Size     int64  `json:"size"`
}

func packageAndPush(ctx context.Context, source, imageName string) (PackageResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return PackageResult{}, fnerrors.Newf("failed to access directory %q: %w", source, err)
	}
	if !info.IsDir() {
		return PackageResult{}, fnerrors.BadInputError("input path %q is not a directory", source)
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return PackageResult{}, fnerrors.Newf("failed to resolve input directory: %w", err)
	}

	workingDir, err := os.MkdirTemp("", "nsc-windows-package-*")
	if err != nil {
		return PackageResult{}, fnerrors.Newf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(workingDir)

	image, err := makeOCIImage(source, workingDir)
	if err != nil {
		return PackageResult{}, fnerrors.Newf("failed to package OCI image: %w", err)
	}
	imageRef, err := pushToNSCR(ctx, image, imageName)
	if err != nil {
		return PackageResult{}, err
	}
	layers, err := image.Layers()
	if err != nil {
		return PackageResult{}, fnerrors.Newf("failed to get image layers: %w", err)
	}
	var totalSize int64
	for _, layer := range layers {
		size, err := layer.Size()
		if err != nil {
			return PackageResult{}, fnerrors.Newf("failed to get layer size: %w", err)
		}
		totalSize += size
	}
	return PackageResult{ImageRef: imageRef, Size: totalSize}, nil
}

func makeOCIImage(source, workingDir string) (v1.Image, error) {
	layerTar := filepath.Join(workingDir, "layer.tar")
	if err := writeTar(layerTar, os.DirFS(source)); err != nil {
		return nil, err
	}
	layer, err := tarball.LayerFromFile(layerTar, tarball.WithMediaType(types.OCILayer), tarball.WithCompressedCaching)
	if err != nil {
		return nil, err
	}
	image, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return nil, err
	}
	return mutate.ConfigMediaType(mutate.MediaType(image, types.OCIManifestSchema1), types.OCIConfigJSON), nil
}

func pushToNSCR(ctx context.Context, image v1.Image, imageName string) (string, error) {
	registry, err := api.GetImageRegistry(ctx, api.Methods)
	if err != nil {
		return "", fnerrors.Newf("could not fetch nscr.io repository: %w", err)
	}
	if registry.NSCR == nil {
		return "", fnerrors.Newf("could not fetch nscr.io repository")
	}

	fullTag := fmt.Sprintf("%s/%s/%s", registry.NSCR.EndpointAddress, registry.NSCR.Repository, imageName)
	tag, err := name.NewTag(fullTag)
	if err != nil {
		return "", fnerrors.BadInputError("invalid image name %q: %v", imageName, err)
	}
	remoteOpts, err := oci.RemoteOptsWithAuth(ctx, oci.RegistryAccess{Keychain: api.DefaultKeychain}, true)
	if err != nil {
		return "", fnerrors.Newf("failed to create remote options: %w", err)
	}
	fmt.Fprintf(console.Stderr(ctx), "Pushing image: %s ...\n", tag.Name())
	if err := remote.Write(tag, image, remoteOpts...); err != nil {
		return "", fnerrors.Newf("failed to push image: %w", err)
	}
	digest, err := image.Digest()
	if err != nil {
		return "", fnerrors.Newf("failed to compute image digest: %w", err)
	}
	return tag.Digest(digest.String()).String(), nil
}

func writeTar(destination string, contents fs.FS) error {
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	tarWriter := tar.NewWriter(out)
	tarErr := tarWriter.AddFS(contents)
	tarCloseErr := tarWriter.Close()
	closeErr := out.Close()
	if tarErr != nil {
		return tarErr
	}
	if tarCloseErr != nil {
		return tarCloseErr
	}
	return closeErr
}
