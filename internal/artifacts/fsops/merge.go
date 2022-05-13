// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fsops

import (
	"context"
	"fmt"
	"io/fs"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Merge(fsys []compute.Computable[fs.FS]) compute.Computable[fs.FS] {
	if len(fsys) == 1 {
		return fsys[0]
	}
	return &merge{fsys: fsys}
}

type merge struct {
	fsys []compute.Computable[fs.FS]

	compute.LocalScoped[fs.FS]
}

func (p *merge) Action() *tasks.ActionEvent {
	return tasks.Action("artifacts.fsops.merge")
}

func (p *merge) Inputs() *compute.In {
	in := compute.Inputs()
	for k, fsys := range p.fsys {
		in = in.Computable(fmt.Sprintf("fsys%d", k), fsys)
	}
	return in
}

func (p *merge) Compute(ctx context.Context, d compute.Resolved) (fs.FS, error) {
	var r memfs.FS

	for k, fsys := range p.fsys {
		res := compute.GetDepValue(d, fsys, fmt.Sprintf("fsys%d", k))

		if err := fnfs.VisitFiles(ctx, res, func(path string, contents bytestream.ByteStream, dirent fs.DirEntry) error {
			st, err := dirent.Info()
			if err != nil {
				return err
			}

			return fnfs.WriteByteStream(ctx, &r, path, contents, st.Mode().Perm())
		}); err != nil {
			return nil, err
		}
	}

	return &r, nil
}
