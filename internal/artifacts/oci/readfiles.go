// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"archive/tar"
	"context"
	"io"
	"io/fs"
	"sort"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func ReadFiles(image NamedImage, paths ...string) compute.Computable[fs.FS] {
	return &readFiles{image: image, paths: paths}
}

type readFiles struct {
	image NamedImage
	paths []string

	compute.LocalScoped[fs.FS]
}

func (r *readFiles) Inputs() *compute.In {
	return compute.Inputs().Strs("paths", r.paths).Computable("image", r.image.Image())
}

func (r *readFiles) Action() *tasks.ActionEvent {
	return tasks.Action("oci.read-image-contents").Arg("image", r.image.Description()).Arg("paths", r.paths)
}

func (r *readFiles) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	return ReadFilesFromImage(compute.MustGetDepValue(deps, r.image.Image(), "image"), r.paths...)
}

type leftMap map[string]struct{}

func (lm leftMap) Has(p string) bool {
	_, ok := lm[p]
	return ok
}

func ReadFilesFromImage(img v1.Image, paths ...string) (*memfs.FS, error) {
	left := leftMap{}
	for _, p := range paths {
		left[p] = struct{}{}
	}

	digest, err := img.Digest()
	if err != nil {
		return nil, fnerrors.BadInputError("failed to obtain image digest: %v", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fnerrors.BadInputError("%s: failed to get layer list", digest)
	}

	var inmem memfs.FS
	for k := len(layers) - 1; k >= 0; k-- {
		if len(left) == 0 {
			break
		}

		readFiles, err := tarfs.ReadFilesInto(&inmem, layers[k].Uncompressed, left)
		if err != nil {
			return nil, fnerrors.BadInputError("%s: failed at layer %d: %w", digest, k, err)
		}

		for _, f := range readFiles {
			delete(left, f)
		}
	}

	if len(left) > 0 {
		var pathsLeft []string
		for p := range left {
			pathsLeft = append(pathsLeft, p)
		}
		sort.Strings(pathsLeft)

		if len(pathsLeft) == 1 {
			return nil, fnerrors.BadInputError("%s: no such file %q", digest, pathsLeft[0])
		}

		return nil, fnerrors.BadInputError("%s: %d files missing, e.g. %q", digest, len(pathsLeft), pathsLeft[0])
	}

	return &inmem, nil
}

func ReadFileFromImage(ctx context.Context, img v1.Image, path string) ([]byte, error) {
	vfs, err := ReadFilesFromImage(img, path)
	if err != nil {
		return nil, err
	}

	return fs.ReadFile(vfs, path)
}

func VisitFilesFromImage(img v1.Image, visitor func(layer, path string, typ byte, contents []byte) error) error {
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

			var contents []byte
			if h.Typeflag == tar.TypeReg {
				contents, err = io.ReadAll(tr)
				if err != nil {
					return err
				}
			} else if h.Typeflag == tar.TypeLink || h.Typeflag == tar.TypeSymlink {
				contents = []byte(h.Linkname)
			}

			if err := visitor(digest.String(), h.Name, h.Typeflag, contents); err != nil {
				return err
			}
		}
	}

	return nil
}
