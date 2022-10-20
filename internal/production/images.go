// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package production

import (
	"context"
	"fmt"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/std/cfg"
)

const (
	Distroless = "distroless"
	Alpine     = "alpine"

	DefaultNonRootUserID = 65000
	DefaultFSGroup       = 65000
)

// Returns a Computable[v1.Image]
func ServerImage(name string, target specs.Platform) (oci.NamedImage, error) {
	base := pins.Server(name)
	if base == nil || base.Base == "" {
		return nil, fnerrors.InternalError("missing base server definition for %q", name)
	}

	serverBase, err := pins.CheckImage(base.Base)
	if err != nil {
		return nil, err
	}

	return oci.ResolveImage(serverBase, target), nil
}

// DevelopmentImage returns a minimal base image where we add tools for development. Use of
// development images is temporary, and likely to only be used when ephemeral containers
// are not available.
func DevelopmentImage(ctx context.Context, name string, env cfg.Context, target build.BuildTarget) (oci.NamedImage, error) {
	base := pins.Server(name)
	if base == nil || base.Base == "" {
		return nil, fnerrors.InternalError("missing base server definition for %q", name)
	}

	if base.NonRootUserID == nil {
		return nil, fnerrors.InternalError("base definition missing userid")
	}

	serverBase, err := pins.CheckImage(base.Base)
	if err != nil {
		return nil, err
	}

	state := llbutil.Image(serverBase, *target.TargetPlatform()).
		With(WithNonRootUserWithUserID(*base.NonRootUserID)).
		Run(llb.Shlex("apk add --no-cache bash")).Root()

	t := build.NewBuildTarget(target.TargetPlatform()).WithTargetName(target.PublishName())

	img, err := buildkit.BuildImage(ctx, env, t.WithSourceLabel("base:"+name), state)
	if err != nil {
		return nil, err
	}

	return oci.MakeNamedImage(fmt.Sprintf("%s + dev tools", serverBase), img), nil
}

func ServerImageLLB(name string, target specs.Platform) (llb.State, error) {
	base := pins.Server(name)
	if base == nil || base.Base == "" {
		return llb.State{}, fnerrors.InternalError("missing base server definition for %q", name)
	}

	serverBase, err := pins.CheckImage(base.Base)
	if err != nil {
		return llb.State{}, err
	}

	return llbutil.Image(serverBase, target), nil
}

func NonRootUser() func(llb.State) llb.State {
	return WithNonRootUserWithUserID(DefaultNonRootUserID)
}

func WithNonRootUserWithUserID(userid int) func(llb.State) llb.State {
	return func(base llb.State) llb.State {
		return base.
			Run(llb.Shlexf("addgroup -g %d nonroot", userid)).
			Run(llb.Shlexf("adduser -h /home/nonroot -D -s /sbin/nologin -G nonroot -u %d nonroot", userid)).
			Root()
	}
}
