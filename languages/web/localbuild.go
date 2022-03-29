// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func ViteLocalBuild(ctx context.Context, resolver workspace.Packages, pkg schema.PackageName, basePath string) (compute.Computable[fs.FS], error) {
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	loc, err := resolver.Resolve(ctx, pkg)
	if err != nil {
		return nil, err
	}

	contents := loc.Module.VersionedFS(loc.Rel(), false)

	return &viteBuild{srcDir: loc.Abs(), contents: contents, basePath: basePath, pkg: pkg}, nil
}

type viteBuild struct {
	srcDir   string
	contents compute.Computable[wscontents.Versioned]
	basePath string

	// For information purposes only.
	pkg schema.PackageName

	compute.LocalScoped[fs.FS]
}

func (vite *viteBuild) Action() *tasks.ActionEvent {
	return tasks.Action("web.vite.build").Arg("builder", "local").Arg("packageName", vite.pkg).Arg("basePath", vite.basePath)
}

func (vite *viteBuild) Inputs() *compute.In {
	return compute.Inputs().Computable("workspace", vite.contents).Indigestible("srcDir", vite.srcDir).Str("basePath", vite.basePath)
}

func (build *viteBuild) Compute(ctx context.Context, _ compute.Resolved) (fs.FS, error) {
	targetDir, err := dirs.CreateUserTempDir("vite", "build")
	if err != nil {
		return nil, err
	}

	// Not 100% correct as srcdir could have changed after we read the workspace contents.
	if err := doBuild(ctx, build.basePath, build.srcDir, targetDir); err != nil {
		return nil, err
	}

	result := fnfs.Local(targetDir)

	// Only initiate a cleanup after we're done compiling.
	compute.On(ctx).Cleanup(tasks.Action("web.vite.build.cleanup"), func(ctx context.Context) error {
		if err := os.RemoveAll(targetDir); err != nil {
			fmt.Fprintln(console.Warnings(ctx), "failed to cleanup target dir", err)
		}
		return nil // Never fail.
	})

	return result, nil
}

func doBuild(ctx context.Context, basePath, srcDir, out string) error {
	var cmd localexec.Command
	// XXX use absolute path to go binary.
	cmd.Label = "vite build"
	cmd.Command = "node_modules/.bin/vite"
	cmd.Args = []string{"build", "--base=" + basePath, "--outDir=" + out, "--emptyOutDir"}
	cmd.Dir = srcDir
	return cmd.Run(ctx)
}