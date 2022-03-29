// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package llbutil

import (
	"context"
	"io/fs"
	"path/filepath"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/fnfs"
)

func WriteFS(ctx context.Context, fsys fs.FS, base llb.State, target string) (llb.State, error) {
	if err := fnfs.VisitFiles(ctx, fsys, func(path string, contents []byte, de fs.DirEntry) error {
		info, err := de.Info()
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