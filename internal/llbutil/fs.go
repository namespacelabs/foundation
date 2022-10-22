// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package llbutil

import (
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnfs"
)

func WriteFS(ctx context.Context, fsys fs.FS, base llb.State, target string) (llb.State, error) {
	if err := fnfs.VisitFiles(ctx, fsys, func(path string, blob bytestream.ByteStream, de fs.DirEntry) error {
		info, err := de.Info()
		if err != nil {
			return err
		}

		contents, err := bytestream.ReadAll(blob)
		if err != nil {
			return err
		}

		base = base.File(llb.Mkfile(filepath.Join(target, path), info.Mode(), contents))
		return nil
	}); err != nil {
		return llb.State{}, err
	}

	return base, nil
}

func AddFile(base llb.State, path string, m fs.FileMode, content []byte) llb.State {
	return base.
		File(llb.Mkdir(filepath.Dir(path), 0755, llb.WithParents(true))).
		File(llb.Mkfile(path, m, content))
}

func AddSerializedJsonAsFile(base llb.State, path string, content any) (llb.State, error) {
	serialized, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return llb.State{}, err
	}

	return AddFile(base, path, 0644, serialized), nil
}
