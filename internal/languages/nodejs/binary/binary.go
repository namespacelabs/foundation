// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/production"
)

const (
	AppRootPath      = "/app"
	backendsConfigFn = "src/config/backends.ns.js"
)

var (
	NodejsExclude = []string{"**/.yarn/cache", "**/.pnp.*"}
)

type nodeJsBinary struct {
	nodejsEnv string
	module    build.Workspace
}

func (n nodeJsBinary) LLB(ctx context.Context, bnj buildNodeJS, conf build.Configuration) (llb.State, []buildkit.LocalContents, error) {
	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return llb.State{}, nil, err
	}

	packageManagerState, err := handlePackageManager(*conf.TargetPlatform(), bnj.config.NodePkgMgr)
	if err != nil {
		return llb.State{}, nil, err
	}

	base := llbutil.Image(nodeImage, *conf.TargetPlatform())

	if packageManagerState.State != nil {
		base = base.With(packageManagerState.State)
	}

	fsys, err := compute.GetValue(ctx, conf.Workspace().VersionedFS(bnj.loc.Rel(), false))
	if err != nil {
		return llb.State{}, nil, err
	}

	baseWithPackageSources := base.File(llb.Mkdir(AppRootPath, 0644))

	local := buildkit.LocalContents{
		Module:          n.module,
		Path:            bnj.loc.Rel(),
		ObserveChanges:  bnj.isFocus,
		ExcludePatterns: NodejsExclude,
	}
	src := buildkit.MakeLocalState(local)

	opts := fnfs.MatcherOpts{
		IncludeFiles:      append([]string{"package.json"}, packageManagerState.Files...),
		ExcludeFilesGlobs: packageManagerState.ExcludePatterns,
	}

	for _, wc := range packageManagerState.WildcardDirectories {
		opts.IncludeFilesGlobs = append(opts.IncludeFilesGlobs, wc+"/**/*")
	}

	m, err := fnfs.NewMatcher(opts)
	if err != nil {
		return llb.State{}, nil, err
	}

	if err := fnfs.VisitFiles(ctx, fsys.FS(), func(path string, bs bytestream.ByteStream, _ fs.DirEntry) error {
		if !m.Excludes(path) && m.Includes(path) {
			baseWithPackageSources = baseWithPackageSources.With(llbutil.CopyFrom(src, path, filepath.Join(AppRootPath, path)))
		}
		return nil
	}); err != nil {
		return llb.State{}, nil, err
	}

	srcWithPkgMgr := baseWithPackageSources.
		Run(llb.Shlexf("%s install", packageManagerState.CLI), llb.Dir(AppRootPath)).
		With(llbutil.CopyFrom(src, ".", AppRootPath))

	srcWithBackendsConfig, err := generateBackendsJs(ctx, srcWithPkgMgr, bnj)
	if err != nil {
		return llb.State{}, nil, err
	}

	var out llb.State
	// The dev and prod builds are different:
	//  - For prod we produce the smallest image, without the package manager and its dependencies.
	//  - For dev we keep the base image with the package manager.
	// This can cause discrepancies between environments however the risk seems to be small.
	if bnj.isDevBuild {
		out = srcWithBackendsConfig
	} else {
		if bnj.config.BuildScript != "" {
			srcWithBackendsConfig = srcWithBackendsConfig.Run(
				llb.Shlexf("%s run %s", packageManagerState.CLI, bnj.config.BuildScript),
				llb.Dir(AppRootPath),
			).Root()
		}

		if bnj.config.BuildOutDir != "" {
			// In this case creating an image with just the built files.
			// TODO: do it outside of the Node.js implementation.
			pathToCopy := filepath.Join(AppRootPath, bnj.config.BuildOutDir)

			out = llb.Scratch().With(llbutil.CopyFrom(srcWithBackendsConfig, pathToCopy, pathToCopy))
		} else {
			// For non-dev builds creating an optimized, small image.
			// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
			out = base.With(
				production.NonRootUser(),
				llbutil.CopyFrom(srcWithBackendsConfig, AppRootPath, AppRootPath),
			)
		}
	}

	out = out.AddEnv("NODE_ENV", n.nodejsEnv)

	return out, []buildkit.LocalContents{local}, nil
}

func generateBackendsJs(ctx context.Context, base llb.State, bnj buildNodeJS) (llb.State, error) {
	if len(bnj.config.InternalDoNotUseBackend) > 0 {
		if _, err := fs.Stat(bnj.loc.Module.ReadOnlyFS(), bnj.loc.Rel(backendsConfigFn)); os.IsNotExist(err) {
			bytes, err := generateBackendsConfig(ctx, bnj.loc, bnj.config.InternalDoNotUseBackend, bnj.assets.IngressFragments, true /* placeholder */)
			if err != nil {
				return llb.State{}, err
			}

			return base, fnerrors.UserError(bnj.loc, `%q must be present in the source tree when Web backends are used. Example content:

%s
`, backendsConfigFn, bytes)
		}

		bytes, err := generateBackendsConfig(ctx, bnj.loc, bnj.config.InternalDoNotUseBackend, bnj.assets.IngressFragments, false /* placeholder */)
		if err != nil {
			return llb.State{}, err
		}

		var fsys memfs.FS
		fsys.Add(backendsConfigFn, bytes)

		return llbutil.WriteFS(ctx, &fsys, base, filepath.Join(AppRootPath))
	} else {
		return base, nil
	}
}
