// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package genbinary

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/pins"
)

func BuildShell(loc pkggraph.Location, plan *schema.ImageBuildPlan_ShellBuild) build.Spec {
	return &buildShell{loc: loc, plan: plan}
}

type buildShell struct {
	loc  pkggraph.Location
	plan *schema.ImageBuildPlan_ShellBuild
}

func (b *buildShell) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	script, err := fs.ReadFile(b.loc.Module.ReadOnlyFS(), filepath.Join(b.loc.Rel(), b.plan.Script))
	if err != nil {
		return nil, fnerrors.Wrapf(b.loc, err, "could not find %q file, please verify that the specified dockerfile path is correct", b.plan.Script)
	}

	image, err := pins.CheckDefault("alpine")
	if err != nil {
		return nil, err
	}

	install := []string{"bash", "curl"}
	install = append(install, b.plan.Install...)

	if conf.TargetPlatform() == nil {
		return nil, fnerrors.InternalError("shell builds require a platform")
	}

	state := llbutil.Image(image, *conf.TargetPlatform()).
		Run(llb.Shlexf("apk add --no-cache %s", strings.Join(install, " "))).
		File(llb.Mkdir(filepath.Dir(b.plan.Script), 0755, llb.WithParents(true))).
		File(llb.Mkfile(b.plan.Script, 0755, script))

	def, err := state.Marshal(ctx)
	if err != nil {
		return nil, fnerrors.InternalError("failed to marshal llb plan: %w", err)
	}

	return buildkit.BuildDefinitionToImage(env, conf, def), nil
}

func (b *buildShell) PlatformIndependent() bool { return false }
