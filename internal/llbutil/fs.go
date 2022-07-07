// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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

func AddSerializedJsonAsFile(base llb.State, path string, content any) (llb.State, error) {
	serialized, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return llb.State{}, err
	}

	return base.
		File(llb.Mkdir(filepath.Dir(path), 0755, llb.WithParents(true))).
		File(llb.Mkfile(path, 0644, serialized)), nil
}
