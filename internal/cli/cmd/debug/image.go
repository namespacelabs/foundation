// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"

	"github.com/dustin/go-humanize"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
)

func newImageCmd() *cobra.Command {
	var (
		insecure bool
		contents bool
		docker   bool
		filename string
	)

	cmd := &cobra.Command{
		Use:   "get-image",
		Short: "Fetches information about an OCI image.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			for _, arg := range args {
				if docker {
					ref, err := oci.ParseRef(arg, insecure)
					if err != nil {
						return err
					}

					img, err := daemon.Image(ref)
					if err != nil {
						return err
					}

					if contents {
						if err := printContents(ctx, img, filename); err != nil {
							return err
						}
					} else {
						if err := printImage(ctx, img); err != nil {
							return err
						}
					}
				} else {
					d, err := fetchImage(arg, insecure, oci.ReadRemoteOpts(ctx)...)
					if err != nil {
						return err
					}

					if contents {
						img, err := d.Image()
						if err != nil {
							return err
						}

						if err := printContents(ctx, img, filename); err != nil {
							return err
						}
					} else {
						if err := printRemote(ctx, d); err != nil {
							return err
						}
					}
				}
			}

			return nil
		}),
	}

	cmd.Flags().BoolVar(&insecure, "insecure", false, "Set to true to access registries over insecure communications.")
	cmd.Flags().BoolVar(&contents, "contents", false, "Set to true to list the contents of the image.")
	cmd.Flags().BoolVar(&docker, "docker", docker, "If true, fetch from docker instead of a registry.")
	cmd.Flags().StringVar(&filename, "filename", "", "If set, outputs the content of the specified file to stdout.")

	return cmd
}

func printRemote(ctx context.Context, d *remote.Descriptor) error {
	fmt.Println(d.Digest.String())
	fmt.Println(d.MediaType)

	img, err := d.Image()
	if err != nil {
		return err
	}

	return printImage(ctx, img)
}

func printImage(ctx context.Context, img v1.Image) error {
	im, err := img.Manifest()
	if err != nil {
		return err
	}

	out := console.Stdout(ctx)

	m := im.Config
	fmt.Fprintf(out, "Image: %s\n", m.Digest.String())
	fmt.Fprintf(out, " size: %v\n", humanize.Bytes(uint64(m.Size)))
	fmt.Fprintf(out, " urls: %v\n", m.URLs)
	fmt.Fprintf(out, " mediaType: %v\n", m.MediaType)
	fmt.Fprintf(out, " annotations: %v\n", m.Annotations)
	fmt.Fprintf(out, " platform: %v\n", m.Platform)

	layers, err := img.Layers()
	if err != nil {
		return err
	}

	var totalSize uint64
	for _, layer := range layers {
		d, _ := layer.Digest()
		size, _ := layer.Size()
		mediaType, _ := layer.MediaType()
		fmt.Fprintf(out, "\n  Layer: %s\n", d)
		fmt.Fprintf(out, "   size: %v\n", humanize.Bytes(uint64(size)))
		fmt.Fprintf(out, "   mediaType: %v\n", mediaType)
		totalSize += uint64(size)
	}

	fmt.Fprintf(out, "\n totalSize: %v\n\n", humanize.Bytes(totalSize))

	return nil
}

func printContents(ctx context.Context, img v1.Image, filename string) error {
	out := console.Stdout(ctx)

	if filename != "" {
		contents, err := oci.ReadFileFromImage(ctx, img, filename)
		if err == nil {
			_, _ = console.Stdout(ctx).Write(contents)
		}
		return err
	}

	var buf bytes.Buffer
	return oci.VisitFilesFromImage(img, func(layer, path string, typ byte, contents []byte) error {
		fmt.Fprintf(&buf, "%s: %s", layer, path)
		switch typ {
		case tar.TypeReg:
			fmt.Fprintf(&buf, " (%d bytes)", len(contents))
		case tar.TypeSymlink:
			fmt.Fprintf(&buf, " --> %s", contents)
		default:
			fmt.Fprintf(&buf, " (%s)", typName(typ))
		}
		fmt.Fprintln(&buf)
		_, _ = buf.WriteTo(out)
		return nil
	})
}

func typName(typ byte) string {
	switch typ {
	case tar.TypeLink:
		return "link"
	case tar.TypeSymlink:
		return "symlink"
	case tar.TypeChar:
		return "char-device"
	case tar.TypeBlock:
		return "block-device"
	case tar.TypeDir:
		return "dir"
	case tar.TypeFifo:
		return "fifo-file"
	case tar.TypeReg:
		return "file"
	default:
		return "unknown"
	}
}
