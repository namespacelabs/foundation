// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"

	"github.com/containerd/stargz-snapshotter/estargz"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

type convertToEstargz struct {
	resolvable compute.Computable[ResolvableImage]

	compute.LocalScoped[ResolvableImage]
}

var _ compute.Computable[ResolvableImage] = &convertToEstargz{}

func (c *convertToEstargz) Action() *tasks.ActionEvent {
	return tasks.Action("oci.convert-to-estargz")
}

func (c *convertToEstargz) Inputs() *compute.In {
	return compute.Inputs().Computable("resolvable", c.resolvable)
}

func (c *convertToEstargz) Compute(ctx context.Context, r compute.Resolved) (ResolvableImage, error) {
	resolved := compute.MustGetDepValue(r, c.resolvable, "resolvable")

	switch raw := resolved.(type) {
	case rawImage:
		newImage, err := convertImageToEstargz(ctx, raw.image)
		if err != nil {
			return nil, err
		}

		return rawImage{newImage}, nil
	case rawImageIndex:
		newIndex, err := convertIndexToEstargz(ctx, raw.index)
		if err != nil {
			return nil, err
		}

		return rawImageIndex{newIndex}, nil
	default:
		return nil, fnerrors.InternalError("unknown ResolvedImage type")
	}
}

func convertImageToEstargz(ctx context.Context, img Image) (Image, error) {
	cf, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}

	ocfg := cf.DeepCopy()
	ocfg.History = nil
	ocfg.RootFS.DiffIDs = nil

	base, err := mutate.ConfigFile(empty.Image, ocfg)
	if err != nil {
		return nil, err
	}

	d, err := img.Manifest()
	if err != nil {
		return nil, err
	}

	eg := executor.New(ctx, "convert-image-to-estargz")

	layers := make([]v1.Layer, len(d.Layers))
	for k := range d.Layers {
		k := k // Capture k.

		eg.Go(func(ctx context.Context) error {
			layer := d.Layers[k]
			l, err := img.LayerByDigest(layer.Digest)
			if err != nil {
				return err
			}

			if layer.Annotations[estargz.TOCJSONDigestAnnotation] != "" {
				layers[k] = l
				return nil
			} else {
				layers[k], err = transformLayer(l)
				return err
			}
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return mutate.AppendLayers(base, layers...)
}

func convertIndexToEstargz(ctx context.Context, index ImageIndex) (ImageIndex, error) {
	m, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}

	eg := executor.New(ctx, "convert-index-to-estargz")

	images := make([]mutate.IndexAddendum, len(m.Manifests))
	for k := range m.Manifests {
		k := k // Capture k.

		eg.Go(func(ctx context.Context) error {
			desc := m.Manifests[k]
			img, err := index.Image(desc.Digest)
			if err != nil {
				return err
			}

			transformed, err := convertImageToEstargz(ctx, img)
			if err != nil {
				return err
			}

			images[k] = mutate.IndexAddendum{
				Add: transformed,
				Descriptor: v1.Descriptor{
					MediaType:   desc.MediaType,
					URLs:        desc.URLs,
					Platform:    desc.Platform,
					Annotations: desc.Annotations,
				},
			}
			return err
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return mutate.AppendManifests(empty.Index, images...), nil
}

func transformLayer(layer v1.Layer) (v1.Layer, error) {
	return tarball.LayerFromOpener(layer.Uncompressed, tarball.WithEstargz, tarball.WithEstargzOptions(estargz.WithCompressionLevel(6)))
}
