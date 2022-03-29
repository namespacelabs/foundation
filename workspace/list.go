// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/wscontents"
)

type SchemaList struct {
	Root *Root

	Locations []fnfs.Location
}

// Recursively visits each non-hidden sub-directory of rootDir, and keeps
// tabs of the schemas on each.
func ListSchemas(ctx context.Context, root *Root) (SchemaList, error) {
	sl := SchemaList{Root: root}

	pl := NewPackageLoader(root)

	visited := map[string]struct{}{} // Map of directory name to presence.

	if err := fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if (path != "." && path[0] == '.') || slices.Contains(wscontents.DirsToAvoid, path) {
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
			ok, err := pl.frontend.HasPackage(ctx, pkg.AsPackageName())
			if err != nil {
				fmt.Fprintf(console.Stderr(ctx), "failed to parse %s: %v\n", dir, err)
				return nil
			}

			if ok {
				sl.Locations = append(sl.Locations, pkg)
			}

			visited[dir] = struct{}{}
		}

		return nil
	}); err != nil {
		return SchemaList{}, err
	}

	return sl, nil
}
