// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazel

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/assert"
)

func TestImageFromLayoutSelectsPlatformAndPreservesImage(t *testing.T) {
	dir := t.TempDir()
	lp, err := layout.Write(dir, empty.Index)
	assert.NilError(t, err)

	var arm v1.Image
	for _, arch := range []string{"amd64", "arm64"} {
		var layers []v1.Layer
		for _, contents := range []string{"base", arch} {
			var data bytes.Buffer
			tw := tar.NewWriter(&data)
			assert.NilError(t, tw.WriteHeader(&tar.Header{Name: "arch", Size: int64(len(contents)), Mode: 0755}))
			_, err := tw.Write([]byte(contents))
			assert.NilError(t, err)
			assert.NilError(t, tw.Close())
			layers = append(layers, static.NewLayer(data.Bytes(), types.OCIUncompressedLayer))
		}
		img, err := mutate.AppendLayers(empty.Image, layers...)
		assert.NilError(t, err)
		config, err := img.ConfigFile()
		assert.NilError(t, err)
		config.OS, config.Architecture = "linux", arch
		config.Config = v1.Config{Entrypoint: []string{"/arch"}, Env: []string{"ARCH=" + arch}, WorkingDir: "/work"}
		img, err = mutate.ConfigFile(img, config)
		assert.NilError(t, err)
		// Exercise layouts without descriptor platforms as well as multi-image selection.
		assert.NilError(t, lp.AppendImage(img))
		if arch == "arm64" {
			arm = img
		}
	}

	archive := filepath.Join(t.TempDir(), "image.tar")
	f, err := os.Create(archive)
	assert.NilError(t, err)
	tw := tar.NewWriter(f)
	assert.NilError(t, tw.AddFS(os.DirFS(dir)))
	assert.NilError(t, tw.Close())
	assert.NilError(t, f.Close())
	extracted := t.TempDir()
	assert.NilError(t, unpackLayout(archive, extracted))
	got, err := imageFromLayout(extracted, specs.Platform{OS: "linux", Architecture: "arm64"})
	assert.NilError(t, err)
	gotDigest, err := got.Digest()
	assert.NilError(t, err)
	wantDigest, err := arm.Digest()
	assert.NilError(t, err)
	assert.Equal(t, gotDigest, wantDigest)
	config, err := got.ConfigFile()
	assert.NilError(t, err)
	assert.DeepEqual(t, config.Config.Entrypoint, []string{"/arch"})
	assert.DeepEqual(t, config.Config.Env, []string{"ARCH=arm64"})
	assert.Equal(t, config.Config.WorkingDir, "/work")
	layers, err := got.Layers()
	assert.NilError(t, err)
	assert.Equal(t, len(layers), 2)

	for _, p := range []specs.Platform{
		{OS: "windows", Architecture: "arm64"},
		{OS: "linux", Architecture: "arm"},
		{OS: "linux", Architecture: "arm64", Variant: "v9"},
	} {
		_, err := imageFromLayout(dir, p)
		assert.ErrorContains(t, err, "no image matching")
	}

	assert.NilError(t, lp.AppendImage(arm))
	_, err = imageFromLayout(dir, specs.Platform{OS: "linux", Architecture: "arm64"})
	assert.ErrorContains(t, err, "multiple images matching")
}

func TestUnpackLayoutRejectsEscapesAndLinks(t *testing.T) {
	for _, header := range []tar.Header{
		{Name: "../escape", Typeflag: tar.TypeReg},
		{Name: "/escape", Typeflag: tar.TypeReg},
		{Name: "symlink", Typeflag: tar.TypeSymlink, Linkname: "../escape"},
		{Name: "hardlink", Typeflag: tar.TypeLink, Linkname: "../escape"},
	} {
		t.Run(header.Name, func(t *testing.T) {
			base := t.TempDir()
			archive := filepath.Join(base, "image.tar")
			f, err := os.Create(archive)
			assert.NilError(t, err)
			tw := tar.NewWriter(f)
			assert.NilError(t, tw.WriteHeader(&header))
			assert.NilError(t, tw.Close())
			assert.NilError(t, f.Close())
			destination := filepath.Join(base, "layout")
			assert.NilError(t, os.Mkdir(destination, 0755))
			assert.Assert(t, unpackLayout(archive, destination) != nil)
			_, err = os.Stat(filepath.Join(base, "escape"))
			assert.Assert(t, os.IsNotExist(err))
		})
	}
}
