// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fsops

import (
	"context"
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/std/tasks"
)

func Prefix(fsys compute.Computable[fs.FS], prefix string) compute.Computable[fs.FS] {
	return &addPrefix{fsys: fsys, prefix: prefix}
}

type addPrefix struct {
	fsys   compute.Computable[fs.FS]
	prefix string

	compute.LocalScoped[fs.FS]
}

func (p *addPrefix) Action() *tasks.ActionEvent {
	return tasks.Action("artifacts.fsops.prefix").Arg("prefix", p.prefix)
}

func (p *addPrefix) Inputs() *compute.In {
	return compute.Inputs().Computable("fsys", p.fsys).Str("prefix", p.prefix)
}

func (p *addPrefix) Compute(ctx context.Context, d compute.Resolved) (fs.FS, error) {
	var r memfs.FS

	return &r, fnfs.VisitFiles(ctx, compute.MustGetDepValue(d, p.fsys, "fsys"), func(path string, contents bytestream.ByteStream, dirent fs.DirEntry) error {
		st, err := dirent.Info()
		if err != nil {
			return err
		}

		return fnfs.WriteByteStream(ctx, &r, filepath.Join(p.prefix, path), contents, st.Mode().Perm())
	})
}
