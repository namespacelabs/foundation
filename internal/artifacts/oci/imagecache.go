// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/compute/cache"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

func RegisterImageCacheable() {
	compute.RegisterCacheable[v1.Layer](layerCacheable{})
	compute.RegisterCacheable[v1.Image](imageCacheable{})
	compute.RegisterCacheable[ResolvableImage](resolvableCacheable{})
}

type baseImage struct {
	rawManifest []byte
	rawConfig   []byte
	manifest    *v1.Manifest
}

type cachedImage struct {
	baseImage
	cache cache.Cache
}

func (li *baseImage) MediaType() (types.MediaType, error) {
	return li.manifest.MediaType, nil
}

func (li *baseImage) Manifest() (*v1.Manifest, error) {
	return li.manifest, nil
}

func (li *baseImage) RawManifest() ([]byte, error) {
	return li.rawManifest, nil
}

func (li *baseImage) RawConfigFile() ([]byte, error) {
	return li.rawConfig, nil
}

func (li *cachedImage) LayerByDigest(h v1.Hash) (partial.CompressedLayer, error) {
	if h == li.manifest.Config.Digest {
		return &compressedBlob{
			cache: li.cache,
			desc:  li.manifest.Config,
		}, nil
	}

	for _, desc := range li.manifest.Layers {
		if h == desc.Digest {
			return &compressedBlob{
				cache: li.cache,
				desc:  desc,
			}, nil
		}
	}

	return nil, fnerrors.InternalError("could not find layer in image: %s", h)
}

type compressedBlob struct {
	cache cache.Cache
	desc  v1.Descriptor
}

func (b *compressedBlob) Digest() (v1.Hash, error) {
	return b.desc.Digest, nil
}

func (b *compressedBlob) Compressed() (io.ReadCloser, error) {
	return b.cache.Blob(schema.Digest(b.desc.Digest))
}

func (b *compressedBlob) Size() (int64, error) {
	return b.desc.Size, nil
}

func (b *compressedBlob) MediaType() (types.MediaType, error) {
	return b.desc.MediaType, nil
}

// Adapted from go-containerregistry.
func writeImageIndex(ctx context.Context, cache cache.Cache, index v1.ImageIndex) (schema.Digest, error) {
	manifest, err := index.IndexManifest()
	if err != nil {
		return schema.Digest{}, err
	}

	// Walk the descriptors and write any v1.Image or v1.ImageIndex that we find.
	// If we come across something we don't expect, just write it as a blob.
	for _, desc := range manifest.Manifests {
		switch {
		case isIndexMediaType(desc.MediaType):
			index, err := index.ImageIndex(desc.Digest)
			if err != nil {
				return schema.Digest{}, err
			}
			if _, err := writeImageIndex(ctx, cache, index); err != nil {
				return schema.Digest{}, err
			}

		case isImageMediaType(desc.MediaType):
			img, err := index.Image(desc.Digest)
			if err != nil {
				return schema.Digest{}, err
			}

			if _, err := (imageCacheable{}).Cache(ctx, cache, img); err != nil {
				return schema.Digest{}, err
			}

		default:
			return schema.Digest{}, fnerrors.BadInputError("don't support caching image indexes with %s", desc.MediaType)
		}
	}

	rawIndex, err := index.RawManifest()
	if err != nil {
		return schema.Digest{}, err
	}

	h, err := index.Digest()
	if err != nil {
		return schema.Digest{}, err
	}

	d := schema.Digest(h)

	return d, cache.WriteBytes(ctx, d, rawIndex)
}

type layerCacheable struct{}

func (layerCacheable) ComputeDigest(ctx context.Context, v interface{}) (schema.Digest, error) {
	_, h, err := ComputeLayerCacheData(v.(v1.Layer))
	return h, err
}

func (layerCacheable) LoadCached(ctx context.Context, c cache.Cache, t compute.CacheableInstance, h schema.Digest) (compute.Result[v1.Layer], error) {
	l, d, err := LoadCachedLayer(ctx, c, h)
	if err != nil {
		return compute.Result[v1.Layer]{}, err
	}
	return compute.Result[v1.Layer]{Value: l, Digest: d}, nil
}

func (layerCacheable) Cache(ctx context.Context, c cache.Cache, v v1.Layer) (schema.Digest, error) {
	return CacheLayer(ctx, c, v)
}

func LoadCachedLayer(ctx context.Context, c cache.Cache, h schema.Digest) (v1.Layer, schema.Digest, error) {
	dataBytes, err := c.Bytes(ctx, h)
	if err != nil {
		return nil, schema.Digest{}, err
	}

	var data cachedLayerData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, schema.Digest{}, err
	}

	l, err := partial.CompressedToLayer(&cachedLayer{cache: c, data: data})
	if err != nil {
		return nil, schema.Digest{}, err
	}

	return l, schema.Digest(data.Digest), nil
}

func CacheLayer(ctx context.Context, c cache.Cache, layer v1.Layer) (schema.Digest, error) {
	d, err := layer.Digest()
	if err != nil {
		return schema.Digest{}, err
	}

	r, err := layer.Compressed()
	if err != nil {
		return schema.Digest{}, err
	}

	if err := c.WriteBlob(ctx, schema.Digest(d), r); err != nil {
		return schema.Digest{}, err
	}

	dataBytes, h, err := ComputeLayerCacheData(layer)
	if err != nil {
		return h, err
	}

	if err := c.WriteBytes(ctx, h, dataBytes); err != nil {
		return h, err
	}

	return h, nil
}

func ComputeLayerCacheData(layer v1.Layer) ([]byte, schema.Digest, error) {
	d, err := layer.Digest()
	if err != nil {
		return nil, schema.Digest{}, err
	}

	size, err := layer.Size()
	if err != nil {
		return nil, schema.Digest{}, err
	}

	mediaType, err := layer.MediaType()
	if err != nil {
		return nil, schema.Digest{}, err
	}

	data := cachedLayerData{
		Digest:    schema.Digest(d),
		Size:      size,
		MediaType: mediaType,
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, schema.Digest{}, err
	}

	h, err := cache.DigestBytes(dataBytes)
	return dataBytes, h, err
}

type cachedLayer struct {
	cache cache.Cache
	data  cachedLayerData
}

type cachedLayerData struct {
	Digest    schema.Digest
	Size      int64
	MediaType types.MediaType
}

func (cl *cachedLayer) Digest() (v1.Hash, error)            { return v1.Hash(cl.data.Digest), nil }
func (cl *cachedLayer) Compressed() (io.ReadCloser, error)  { return cl.cache.Blob(cl.data.Digest) }
func (cl *cachedLayer) Size() (int64, error)                { return cl.data.Size, nil }
func (cl *cachedLayer) MediaType() (types.MediaType, error) { return cl.data.MediaType, nil }

type imageCacheable struct{}

func (imageCacheable) ComputeDigest(ctx context.Context, v interface{}) (schema.Digest, error) {
	d, err := v.(v1.Image).Digest()
	return schema.Digest(d), err
}

func (imageCacheable) LoadCached(ctx context.Context, c cache.Cache, _ compute.CacheableInstance, h schema.Digest) (compute.Result[v1.Image], error) {
	img, err := loadFromCache(ctx, c, v1.Hash(h))
	if err != nil {
		return compute.Result[v1.Image]{}, err
	}

	if img == nil {
		return compute.NoResult[v1.Image]()
	}

	return compute.Result[v1.Image]{
		Value:  img,
		Digest: h,
	}, nil
}

func (imageCacheable) Cache(ctx context.Context, c cache.Cache, img v1.Image) (schema.Digest, error) {
	d, err := img.Digest()
	if err != nil {
		return schema.Digest{}, err
	}

	if err := writeImage(ctx, c, img); err != nil {
		return schema.Digest{}, err
	}

	return schema.Digest(d), nil
}

func loadCachedManifest(ctx context.Context, cache cache.Cache, d v1.Hash) ([]byte, *v1.Manifest, error) {
	rawManifest, err := cache.Bytes(ctx, schema.Digest(d))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fnerrors.InternalError("cached image manifest failed to load: %w", err)
	}

	m, err := v1.ParseManifest(bytes.NewReader(rawManifest))
	if err != nil {
		return nil, nil, fnerrors.InternalError("cached image manifest failed to parse: %w", err)
	}

	return rawManifest, m, nil
}

func loadFromCache(ctx context.Context, cache cache.Cache, d v1.Hash) (v1.Image, error) {
	rawManifest, m, err := loadCachedManifest(ctx, cache, d)
	if err != nil {
		return nil, err
	}

	if m == nil {
		return nil, nil
	}

	if isImageMediaType(m.MediaType) {
		rawConfig, err := cache.Bytes(ctx, schema.Digest(m.Config.Digest))
		if err != nil {
			return nil, err
		}

		ci := &cachedImage{cache: cache}
		ci.rawManifest = rawManifest
		ci.rawConfig = rawConfig
		ci.manifest = m
		return partial.CompressedToImage(ci)
	}

	return nil, nil
}

// This is a fairly strict check, it will need revisiting.
func platformMatches(stored, requested *v1.Platform) bool {
	if stored == nil || requested == nil {
		return false
	}
	return stored.Architecture == requested.Architecture &&
		stored.OS == requested.OS &&
		(requested.Variant == "" || stored.Variant == requested.Variant)
}

type resolvableCacheable struct{}

func (resolvableCacheable) ComputeDigest(ctx context.Context, v interface{}) (schema.Digest, error) {
	return v.(ResolvableImage).Digest()
}

func (resolvableCacheable) LoadCached(ctx context.Context, c cache.Cache, t compute.CacheableInstance, h schema.Digest) (compute.Result[ResolvableImage], error) {
	r, err := loadCachedResolvable(ctx, c, v1.Hash(h))
	if err != nil {
		return compute.Result[ResolvableImage]{}, err
	}

	if r == nil {
		return compute.NoResult[ResolvableImage]()
	}

	return compute.Result[ResolvableImage]{
		Value:  r,
		Digest: h,
	}, nil
}

func (resolvableCacheable) Cache(ctx context.Context, c cache.Cache, r ResolvableImage) (schema.Digest, error) {
	return r.cache(ctx, c)
}

func isIndexMediaType(md types.MediaType) bool {
	return md == types.DockerManifestList || md == types.OCIImageIndex
}

func isImageMediaType(md types.MediaType) bool {
	return md == types.DockerManifestSchema2 || md == types.OCIManifestSchema1
}

func loadCachedResolvable(ctx context.Context, cache cache.Cache, h v1.Hash) (ResolvableImage, error) {
	rawManifest, m, err := loadCachedManifest(ctx, cache, v1.Hash(h))
	if err != nil {
		return nil, err
	}

	if m == nil {
		return nil, nil
	}

	switch {
	case isIndexMediaType(m.MediaType):
		idx, err := v1.ParseIndexManifest(bytes.NewReader(rawManifest))
		if err != nil {
			return nil, fnerrors.InternalError("cached image index manifest failed to load: %w", err)
		}

		// XXX parallelize?
		children := make([]ResolvableImage, len(idx.Manifests))
		for k, m := range idx.Manifests {
			child, err := loadCachedResolvable(ctx, cache, m.Digest)
			if err != nil {
				return nil, fnerrors.InternalError("index: failed to load %s: %w", m.Digest, err)
			}
			// If the image is missing, the cache is incomplete, and can't make use of this index.
			if child == nil {
				return nil, nil
			}
			children[k] = child
		}

		return rawImageIndex{index: cachedIndex{rawManifest: rawManifest, parsed: idx, cache: cache, children: children}}, nil

	case isImageMediaType(m.MediaType):
		image, err := loadFromCache(ctx, cache, h)
		if err != nil {
			return nil, err
		}

		return rawImage{image: image}, nil
	}

	return nil, nil
}

type cachedIndex struct {
	rawManifest []byte
	parsed      *v1.IndexManifest
	cache       cache.Cache
	children    []ResolvableImage
}

func (c cachedIndex) MediaType() (types.MediaType, error) {
	if string(c.parsed.MediaType) != "" {
		return c.parsed.MediaType, nil
	}
	return types.OCIImageIndex, nil
}

func (c cachedIndex) Digest() (v1.Hash, error) {
	return partial.Digest(c)
}

func (c cachedIndex) Size() (int64, error) {
	return partial.Size(c)
}

func (c cachedIndex) IndexManifest() (*v1.IndexManifest, error) {
	return c.parsed, nil
}

func (c cachedIndex) RawManifest() ([]byte, error) {
	return c.rawManifest, nil
}

func (c cachedIndex) Image(h v1.Hash) (v1.Image, error) {
	r, err := c.childByHash(h)
	if err != nil {
		return nil, err
	}
	return r.Image()
}

func (c cachedIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	r, err := c.childByHash(h)
	if err != nil {
		return nil, err
	}
	return r.ImageIndex()
}

func (c cachedIndex) childByHash(h v1.Hash) (ResolvableImage, error) {
	for k, m := range c.parsed.Manifests {
		if m.Digest == h {
			return c.children[k], nil
		}
	}
	return nil, fnerrors.BadInputError("no such child with hash %s", h)
}

// Adapted from go-containerregistry.
func writeImage(ctx context.Context, cache cache.Cache, img Image) error {
	digest, err := img.Digest()
	if err != nil {
		return err
	}

	return tasks.Action("oci.image-cache.write").LogLevel(2).Arg("digest", digest.String()).Run(ctx, func(ctx context.Context) error {
		manifest, err := img.RawManifest()
		if err != nil {
			return err
		}

		cfgBlob, err := img.RawConfigFile()
		if err != nil {
			return err
		}

		layers, err := img.Layers()
		if err != nil {
			return err
		}

		totalBytes := uint64(len(manifest) + len(cfgBlob))

		for _, layer := range layers {
			size, err := layer.Size()
			if err != nil {
				return err
			}

			totalBytes += uint64(size)
		}

		progress := artifacts.NewProgressWriter(totalBytes, nil)
		tasks.Attachments(ctx).SetProgress(progress)

		// Write the layers concurrently.
		eg := executor.New(ctx, "oci.image-cache.write-layers")
		for _, layer := range layers {
			layer := layer // Capture layer.
			eg.Go(func(ctx context.Context) error {
				return writeLayer(ctx, cache, layer, progress)
			})
		}
		if err := eg.Wait(); err != nil {
			return err
		}

		cfgName, err := img.ConfigName()
		if err != nil {
			return err
		}

		if err := cache.WriteBlob(ctx, schema.Digest(cfgName), progress.WrapBytesAsReader(cfgBlob)); err != nil {
			return err
		}

		return cache.WriteBlob(ctx, schema.Digest(digest), progress.WrapBytesAsReader(manifest))
	})
}

func writeLayer(ctx context.Context, cache cache.Cache, layer v1.Layer, progress artifacts.ProgressWriter) error {
	d, err := layer.Digest()
	if err != nil {
		return err
	}

	r, err := layer.Compressed()
	if err != nil {
		return err
	}

	return cache.WriteBlob(ctx, schema.Digest(d), progress.WrapReader(r))
}
