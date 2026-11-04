// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazel

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/layout"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

type Image struct {
	Workspace build.Workspace
	Target    string
	Platforms map[string]string
}

func (im Image) PlatformIndependent() bool { return false }

func (im Image) Description() string { return fmt.Sprintf("Bazel image %s", im.Target) }

func (im Image) BuildImage(ctx context.Context, _ pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	if !strings.HasPrefix(im.Target, "//") && !strings.HasPrefix(im.Target, "@") {
		return nil, fnerrors.BadInputError("bazel_build.target must be an absolute Bazel label, got %q", im.Target)
	}
	if conf.TargetPlatform() == nil {
		return nil, fnerrors.InternalError("bazel_build: target platform is missing")
	}
	p := *conf.TargetPlatform()
	bazelPlatform := ""
	if len(im.Platforms) > 0 {
		bazelPlatform = im.Platforms[platform.FormatPlatform(p)]
		if bazelPlatform == "" {
			return nil, fnerrors.BadInputError("bazel_build: no Bazel platform configured for %s", platform.FormatPlatform(p))
		}
	}
	output, err := NewBuilder().AddTarget(ctx, Target{
		WorkspaceAbs: im.Workspace.Abs(),
		Label:        im.Target,
		Platform:     bazelPlatform,
		OutputGroup:  "oci_tarball",
	})
	if err != nil {
		return nil, err
	}

	return compute.Transform("import-image", output, func(ctx context.Context, output string) (oci.Image, error) {
		// rules_img directory layouts can contain symlinked blobs, which the OCI
		// reader rejects. Tar outputs embed the bytes and also isolate us from
		// subsequent Bazel builds replacing their outputs.
		dir, err := dirs.CreateUserTempDir("bazel", "image")
		if err != nil {
			return nil, err
		}
		compute.On(ctx).Cleanup(tasks.Action("bazel.image.cleanup"), func(context.Context) error {
			return os.RemoveAll(dir)
		})
		if err := unpackLayout(output, dir); err != nil {
			return nil, fnerrors.Newf("bazel_build %s: extracting OCI layout: %w", im.Target, err)
		}
		img, err := imageFromLayout(dir, p)
		if err != nil {
			return nil, fnerrors.Newf("bazel_build %s: %w", im.Target, err)
		}
		return img, nil
	}), nil
}

func unpackLayout(source, dir string) error {
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(h.Name, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := root.MkdirAll(filepath.Dir(h.Name), 0755); err != nil {
				return err
			}
			out, err := root.OpenFile(h.Name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			closeErr := out.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fnerrors.BadInputError("unsupported OCI layout tar entry %q (type %d)", h.Name, h.Typeflag)
		}
	}
}

func imageFromLayout(dir string, requested specs.Platform) (oci.Image, error) {
	index, err := layout.ImageIndexFromPath(dir)
	if err != nil {
		return nil, err
	}
	manifest, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}
	var selected oci.Image
	for _, desc := range manifest.Manifests {
		if !desc.MediaType.IsImage() {
			continue
		}
		img, err := index.Image(desc.Digest)
		if err != nil {
			return nil, err
		}
		config, err := img.ConfigFile()
		if err != nil {
			return nil, err
		}
		// A single-image layout need not have a platform on its descriptor.
		// Validate the image config even in that case instead of assuming amd64.
		if config.OS != requested.OS || config.Architecture != requested.Architecture ||
			(requested.Variant != "" && config.Variant != requested.Variant) {
			continue
		}
		if selected != nil {
			return nil, fnerrors.BadInputError("OCI layout has multiple images matching %s", platform.FormatPlatform(requested))
		}
		selected = img
	}
	if selected == nil {
		return nil, fnerrors.BadInputError("OCI layout has no image matching %s", platform.FormatPlatform(requested))
	}
	return selected, nil
}
