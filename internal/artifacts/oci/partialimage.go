// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
)

// We've forked compressedImageExtender so we can identify a previous loaded
// cached image, and avoid re-writing.
//
// Copyright 2018 Google LLC All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");

// cachedImage implements v1.Image by extending CompressedImageCore with the
// appropriate methods computed from the minimal core.
type cachedImage struct {
	partial.CompressedImageCore
}

// Assert that our extender type completes the v1.Image interface
var _ v1.Image = (*cachedImage)(nil)

// Digest implements v1.Image
func (i *cachedImage) Digest() (v1.Hash, error) {
	return partial.Digest(i)
}

// ConfigName implements v1.Image
func (i *cachedImage) ConfigName() (v1.Hash, error) {
	return partial.ConfigName(i)
}

// Layers implements v1.Image
func (i *cachedImage) Layers() ([]v1.Layer, error) {
	hs, err := partial.FSLayers(i)
	if err != nil {
		return nil, err
	}
	ls := make([]v1.Layer, 0, len(hs))
	for _, h := range hs {
		l, err := i.LayerByDigest(h)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)
	}
	return ls, nil
}

// LayerByDigest implements v1.Image
func (i *cachedImage) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	cl, err := i.CompressedImageCore.LayerByDigest(h)
	if err != nil {
		return nil, err
	}
	return partial.CompressedToLayer(cl)
}

// LayerByDiffID implements v1.Image
func (i *cachedImage) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	h, err := partial.DiffIDToBlob(i, h)
	if err != nil {
		return nil, err
	}
	return i.LayerByDigest(h)
}

// ConfigFile implements v1.Image
func (i *cachedImage) ConfigFile() (*v1.ConfigFile, error) {
	return partial.ConfigFile(i)
}

// Manifest implements v1.Image
func (i *cachedImage) Manifest() (*v1.Manifest, error) {
	return partial.Manifest(i)
}

// Size implements v1.Image
func (i *cachedImage) Size() (int64, error) {
	return partial.Size(i)
}
