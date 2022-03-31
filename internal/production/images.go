// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package production

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/pins"
)

const (
	Distroless = "distroless"
	Alpine     = "alpine"

	NonRootUserID = 65000
)

// Returns a Computable[v1.Image]
func ServerImage(name string, target specs.Platform) (compute.Computable[oci.Image], error) {
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
func DevelopmentImage(ctx context.Context, name string, env ops.Environment, target specs.Platform) (compute.Computable[oci.Image], error) {
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

	state := prepareImage(llbutil.Image(serverBase, target), *base.NonRootUserID)
	state = state.Run(llb.Shlex("apk add --no-cache bash")).Root()

	return buildkit.LLBToImage(ctx, env, &target, state)
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

func PrepareImage(base llb.State, platform specs.Platform) llb.State {
	return base.
		Run(llb.Shlexf("addgroup -g %d nonroot", NonRootUserID)).
		Run(llb.Shlexf("adduser -h /home/nonroot -D -s /sbin/nologin -G nonroot -u %d nonroot", NonRootUserID)).
		Root()
}

func prepareImage(base llb.State, userid int) llb.State {
	return base.
		Run(llb.Shlexf("addgroup -g %d nonroot", userid)).
		Run(llb.Shlexf("adduser -h /home/nonroot -D -s /sbin/nologin -G nonroot -u %d nonroot", userid)).
		Root()
}
