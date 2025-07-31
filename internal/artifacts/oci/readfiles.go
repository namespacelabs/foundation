// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"archive/tar"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
)

func ImageAsFS(image v1.Image) tarfs.FS {
	return tarfs.FS{TarStream: func() (io.ReadCloser, error) { return mutate.Extract(image), nil }}
}

func ReadFilesFromImage(img v1.Image, visitor func(layer, path string, typ byte, contents []byte) error) error {
	return VisitFilesFromImage(img, func(layer string, h *tar.Header, reader io.Reader) error {
		var contents []byte
		switch {
		case h.Typeflag == tar.TypeReg:
			fileContents, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			contents = fileContents
		case h.Typeflag == tar.TypeLink || h.Typeflag == tar.TypeSymlink:
			contents = []byte(h.Linkname)
		}

		if err := visitor(layer, h.Name, h.Typeflag, contents); err != nil {
			return err
		}

		return nil
	})
}

func VisitFilesFromImage(img v1.Image, visitor func(layer string, header *tar.Header, reader io.Reader) error) error {
	layers, err := img.Layers()
	if err != nil {
		return err
	}

	for _, layer := range layers {
		digest, err := layer.Digest()
		if err != nil {
			return err
		}

		r, err := layer.Uncompressed()
		if err != nil {
			return err
		}

		defer r.Close()

		tr := tar.NewReader(r)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}

			if err := visitor(digest.String(), h, tr); err != nil {
				return err
			}
		}
	}

	return nil
}
