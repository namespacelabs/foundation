// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/schema"
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

type HashPathOpts struct {
	// Also include links in the hashed values. Links are hashed by link NAME, link target is not read
	IncludeLinkNames bool

	// Treat the specified path not existing as an empty directory rather than an error
	AllowInvalidPath bool
}

// HashPathInImage iterates over all files in the image under path, and calculates a hash of its contents
// It takes into account filenames and file contents, other file metadata is ignored
// The algorithm is based on https://github.com/golang/mod/blob/ce943fd02449f621243c9ea6e64098e84752b92b/sumdb/dirhash/hash.go#L44
func HashPathInImage(img v1.Image, path string, opts HashPathOpts) (schema.Digest, error) {
	path = filepath.Clean(path)

	// Trim leading slash to match tar paths
	path, ok := strings.CutPrefix(path, "/")
	if !ok {
		return schema.Digest{}, fmt.Errorf("path must be absolute, got: %s", path)
	}

	// file path -> hash
	fileHashes := map[string]string{}

	pathExists := false

	// Collect all relevant files with their hashes
	if err := VisitFilesFromImage(img, func(layer string, h *tar.Header, reader io.Reader) error {
		// Skip files not under path, this matches exact matches (e.g. /etc/passwd) or any items under a directory (e.g. /etc/)
		// hard-coded separator because tar requires forward slashes
		if h.Name != path && !strings.HasPrefix(h.Name, path+"/") {
			return nil
		}

		pathExists = true

		hf := sha256.New()

		switch h.Typeflag {
		case tar.TypeReg:
			_, err := io.Copy(hf, reader)
			if err != nil {
				return err
			}
		case tar.TypeLink, tar.TypeSymlink:
			if !opts.IncludeLinkNames {
				return nil
			}
			hf.Write([]byte(h.Linkname))
		default:
			// Not a file or link, skip
			return nil
		}

		fileHashes[h.Name] = hex.EncodeToString(hf.Sum(nil))

		return nil
	}); err != nil {
		return schema.Digest{}, err
	}

	if !pathExists && !opts.AllowInvalidPath {
		return schema.Digest{}, fmt.Errorf("path not found: %s", path)
	}

	// Then iterate over the collected hashes in filename order and calculate the overall hash
	sortedFiles := sortedKeys(fileHashes)
	h := sha256.New()

	for _, file := range sortedFiles {
		fileHash := fileHashes[file]
		if _, err := fmt.Fprintf(h, "%s  %s\n", fileHash, file); err != nil {
			return schema.Digest{}, err
		}
	}

	return schema.FromHash("sha256", h), nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
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
