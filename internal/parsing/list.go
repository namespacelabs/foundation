// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package parsing

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/cfg"
)

type SchemaList struct {
	Root *Root

	Locations []fnfs.Location
	Types     []PackageType
}

// Returns a list of all of the schema definitions found under root.
func ListSchemas(ctx context.Context, env cfg.Context, root *Root) (SchemaList, error) {
	sl := SchemaList{Root: root}

	pl := NewPackageLoader(env)

	visited := map[string]struct{}{} // Map of directory name to presence.

	if err := fnfs.WalkDir(root.ReadOnlyFS(), ".", func(path string, d fs.DirEntry) error {
		if d.IsDir() {
			if dirs.IsExcludedAsSource(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}

		// Is there a least a .cue file in the directory?
		if filepath.Ext(d.Name()) == ".cue" {
			dir := filepath.Dir(path)
			if _, ok := visited[dir]; ok {
				return nil
			}

			pkg := root.RelPackage(dir)

			ptype, err := pl.frontend.GuessPackageType(ctx, pkg.AsPackageName())
			if err != nil {
				fmt.Fprintf(console.Stderr(ctx), "failed to parse %s: %v\n", dir, err)
				return nil
			}

			if ptype != PackageType_None {
				sl.Locations = append(sl.Locations, pkg)
				sl.Types = append(sl.Types, ptype)
			}

			visited[dir] = struct{}{}
		}

		return nil
	}); err != nil {
		return SchemaList{}, err
	}

	return sl, nil
}
