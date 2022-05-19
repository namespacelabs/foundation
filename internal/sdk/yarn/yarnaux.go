// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package yarn

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/languages/nodejs/yarnplugin"
	"namespacelabs.dev/foundation/workspace/compute"
)

// This package provides auxiliary files for invoking Yarn: the Foundation plugin and ".yarnrc.yml",
// so they don't need to be submitted to the user codebase.

const (
	PluginFn      = "plugin-foundation.cjs"
	YarnRcFn      = ".yarnrc.yml"
	yarnRcContent = `nodeLinker: node-modules

npmScopes:
  namespacelabs:
    npmRegistryServer: "https://us-npm.pkg.dev/foundation-344819/npm-prebuilts/"
`
)

// Returns the directory with all the files
func EnsureYarnAuxFilesDir(ctx context.Context) (string, error) {
	v, err := compute.GetValue(ctx, computable(ctx))
	if err != nil {
		return "", err
	}
	return v.Files, nil
}

func YarnAuxFiles() fs.FS {
	var fsys memfs.FS
	fsys.Add(PluginFn, yarnplugin.PluginContent())
	fsys.Add(YarnRcFn, []byte(yarnRcContent))
	return &fsys
}

func computable(ctx context.Context) compute.Computable[unpack.Unpacked] {
	fsys := YarnAuxFiles()
	return unpack.Unpack("yarn-plugin", compute.Precomputed(fsys, fsys.(*memfs.FS).ComputeDigest))
}
