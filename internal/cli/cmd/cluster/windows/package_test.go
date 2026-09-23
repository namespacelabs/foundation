// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package windows

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestOCIImageContainsDirectoryContents(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(source, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	fileContents := []byte("OCI layer contents")
	if err := os.WriteFile(filepath.Join(source, "nested", "file.txt"), fileContents, 0o644); err != nil {
		t.Fatal(err)
	}

	workingDir := t.TempDir()
	image, err := makeOCIImage(source, workingDir)
	if err != nil {
		t.Fatal(err)
	}
	mediaType, err := image.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	if mediaType != types.OCIManifestSchema1 {
		t.Fatalf("image media type = %q, want %q", mediaType, types.OCIManifestSchema1)
	}
	layers, err := image.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 1 {
		t.Fatalf("layer count = %d, want 1", len(layers))
	}
	layerType, err := layers[0].MediaType()
	if err != nil {
		t.Fatal(err)
	}
	if layerType != types.OCILayer {
		t.Fatalf("layer media type = %q, want %q", layerType, types.OCILayer)
	}
	layerContents, err := layers[0].Uncompressed()
	if err != nil {
		t.Fatal(err)
	}
	layerEntries := readTar(t, tar.NewReader(layerContents))
	if err := layerContents.Close(); err != nil {
		t.Fatal(err)
	}
	if contents := layerEntries["nested/file.txt"]; !bytes.Equal(contents, fileContents) {
		t.Fatalf("layer file contents = %q, want %q", contents, fileContents)
	}
	if _, ok := layerEntries["empty/"]; !ok {
		t.Fatal("layer does not contain empty directory")
	}
}

func readTar(t *testing.T, r *tar.Reader) map[string][]byte {
	t.Helper()
	entries := map[string][]byte{}
	for {
		header, err := r.Next()
		if err == io.EOF {
			return entries
		}
		if err != nil {
			t.Fatal(err)
		}
		switch header.Typeflag {
		case tar.TypeReg:
			contents, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			entries[header.Name] = contents
		case tar.TypeDir:
			entries[header.Name] = nil
		}
	}
}
