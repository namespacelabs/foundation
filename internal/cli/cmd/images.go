// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/mattn/go-zglob"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/runtime/docker"
)

func NewImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "images",
		Short:  "Image related functionality.",
		Hidden: true,
	}

	var image, target string
	var insecure bool
	var extract []string

	unpack := &cobra.Command{
		Use:   "unpack --image <image-ref> --target <path/to/dir>",
		Short: "Unpack an image to the local filesystem.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			var globs []fnfs.HasMatch
			for _, glob := range extract {
				x, err := zglob.New(glob)
				if err != nil {
					return err
				}
				globs = append(globs, x)
			}

			platform := docker.HostPlatform()

			img, err := compute.GetValue(ctx, oci.ImageP(image, &platform, oci.ResolveOpts{InsecureRegistry: insecure}))
			if err != nil {
				return err
			}

			dst := fnfs.ReadWriteLocalFS(target, fnfs.AnnounceWrites(console.Stdout(ctx)))

			if err := dst.MkdirAll(".", 0700); err != nil {
				return err
			}

			tr := tar.NewReader(mutate.Extract(img))
			for {
				h, err := tr.Next()
				if err == io.EOF {
					break
				} else if err != nil {
					return fnerrors.BadInputError("unexpected error: %v", err)
				}

				clean := filepath.Clean(h.Name)
				if !matchAny(globs, clean) {
					continue
				}

				switch h.Typeflag {
				case tar.TypeDir:
					if err := dst.MkdirAll(clean, 0700); err != nil {
						return err
					}

				case tar.TypeReg:
					w, err := dst.OpenWrite(clean, h.FileInfo().Mode().Perm())
					if err != nil {
						return err
					}
					_, copyErr := io.Copy(w, tr)
					closeErr := w.Close()
					if copyErr == nil {
						copyErr = closeErr
					}
					if copyErr != nil {
						return copyErr
					}

				default:
					fmt.Fprintf(console.Warnings(ctx), "ignoring %q (%v)\n", clean, h.Typeflag)
				}
			}

			return nil
		}),
	}

	unpack.Flags().StringVar(&image, "image", "", "Which image to unpack.")
	unpack.Flags().StringVar(&target, "target", "", "Where the image should be unpacked to.")
	unpack.Flags().StringArrayVar(&extract, "extract", nil, "If set, limits the paths being exported to the specified list.")
	unpack.Flags().BoolVar(&insecure, "insecure", false, "Access to the registry is insecure.")

	_ = unpack.MarkFlagRequired("image")
	_ = unpack.MarkFlagRequired("target")

	cmd.AddCommand(unpack)

	return cmd
}

func matchAny(globs []fnfs.HasMatch, path string) bool {
	if len(globs) == 0 {
		return true
	}

	for _, glob := range globs {
		if glob.Match(path) {
			return true
		}
	}
	return false
}
