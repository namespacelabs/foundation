// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"bytes"
	"context"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/contentstore"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

func RegisterImageDigesters() {
	compute.RegisterDigester[v1.Layer](layerDigester{})
	compute.RegisterDigester[v1.Image](imageDigester{})
	compute.RegisterDigester[ResolvableImage](resolvableDigester{})
}

func platformMatches(stored, requested *v1.Platform) bool {
	if stored == nil || requested == nil {
		return false
	}
	return stored.Architecture == requested.Architecture &&
		stored.OS == requested.OS &&
		(requested.Variant == "" || stored.Variant == requested.Variant)
}

func isIndexMediaType(mediaType types.MediaType) bool {
	return mediaType == types.DockerManifestList || mediaType == types.OCIImageIndex
}

func isImageMediaType(mediaType types.MediaType) bool {
	return mediaType == types.DockerManifestSchema2 || mediaType == types.OCIManifestSchema1
}

type layerDigester struct{}

func (layerDigester) ComputeDigest(_ context.Context, layer v1.Layer) (schema.Digest, error) {
	digest, err := layer.Digest()
	return schema.Digest(digest), err
}

type imageDigester struct{}

func (imageDigester) ComputeDigest(_ context.Context, image v1.Image) (schema.Digest, error) {
	digest, err := image.Digest()
	return schema.Digest(digest), err
}

type resolvableDigester struct{}

func (resolvableDigester) ComputeDigest(_ context.Context, image ResolvableImage) (schema.Digest, error) {
	return image.Digest()
}

type baseImage struct {
	rawManifest []byte
	rawConfig   []byte
	manifest    *v1.Manifest
}

type localImageCore struct {
	baseImage
	store contentstore.Store
}

func (image *baseImage) MediaType() (types.MediaType, error) {
	return image.manifest.MediaType, nil
}

func (image *baseImage) Manifest() (*v1.Manifest, error) {
	return image.manifest, nil
}

func (image *baseImage) RawManifest() ([]byte, error) {
	return image.rawManifest, nil
}

func (image *baseImage) RawConfigFile() ([]byte, error) {
	return image.rawConfig, nil
}

func (image *localImageCore) LayerByDigest(digest v1.Hash) (partial.CompressedLayer, error) {
	if digest == image.manifest.Config.Digest {
		return &localBlob{store: image.store, desc: image.manifest.Config}, nil
	}

	for _, desc := range image.manifest.Layers {
		if digest == desc.Digest {
			return &localBlob{store: image.store, desc: desc}, nil
		}
	}

	return nil, fnerrors.InternalError("could not find layer in image: %s", digest)
}

type localBlob struct {
	store contentstore.Store
	desc  v1.Descriptor
}

func (blob *localBlob) Digest() (v1.Hash, error) { return blob.desc.Digest, nil }
func (blob *localBlob) Compressed() (io.ReadCloser, error) {
	return blob.store.Blob(schema.Digest(blob.desc.Digest))
}
func (blob *localBlob) Size() (int64, error)                { return blob.desc.Size, nil }
func (blob *localBlob) MediaType() (types.MediaType, error) { return blob.desc.MediaType, nil }

func loadLocalImage(ctx context.Context, store contentstore.Store, digest v1.Hash) (Image, error) {
	rawManifest, err := store.Bytes(ctx, schema.Digest(digest))
	if err != nil {
		return nil, err
	}
	manifest, err := v1.ParseManifest(bytes.NewReader(rawManifest))
	if err != nil {
		return nil, fnerrors.InternalError("stored image manifest failed to parse: %w", err)
	}
	rawConfig, err := store.Bytes(ctx, schema.Digest(manifest.Config.Digest))
	if err != nil {
		return nil, err
	}

	core := &localImageCore{store: store}
	core.rawManifest = rawManifest
	core.rawConfig = rawConfig
	core.manifest = manifest
	return &localImage{CompressedImageCore: core}, nil
}

func writeImage(ctx context.Context, store contentstore.Store, image Image) error {
	digest, err := image.Digest()
	if err != nil {
		return err
	}
	manifest, err := image.RawManifest()
	if err != nil {
		return err
	}
	config, err := image.RawConfigFile()
	if err != nil {
		return err
	}
	configName, err := image.ConfigName()
	if err != nil {
		return err
	}
	layers, err := image.Layers()
	if err != nil {
		return err
	}

	totalBytes := uint64(len(manifest) + len(config))
	for _, layer := range layers {
		size, err := layer.Size()
		if err != nil {
			return err
		}
		totalBytes += uint64(size)
	}

	progress := artifacts.NewProgressWriter(totalBytes, nil)
	tasks.Attachments(ctx).SetProgress(progress)
	eg := executor.New(ctx, "oci.image-artifacts.write")
	for _, layer := range layers {
		layer := layer
		eg.Go(func(ctx context.Context) error {
			digest, err := layer.Digest()
			if err != nil {
				return err
			}
			contents, err := layer.Compressed()
			if err != nil {
				return err
			}
			return store.WriteBlob(ctx, schema.Digest(digest), progress.WrapReader(contents))
		})
	}
	eg.Go(func(ctx context.Context) error {
		return store.WriteBlob(ctx, schema.Digest(configName), progress.WrapBytesAsReader(config))
	})
	eg.Go(func(ctx context.Context) error {
		return store.WriteBlob(ctx, schema.Digest(digest), progress.WrapBytesAsReader(manifest))
	})
	return eg.Wait()
}
