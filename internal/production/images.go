// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package production

import (
	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/pins"
)

const Distroless = "distroless"
const NonRootUserID = 65000

// Returns a Computable[v1.Image]
func ServerImage(name string, target specs.Platform) (compute.Computable[oci.Image], error) {
	base := pins.Server(name)
	if base == nil || base.Base == "" {
		return nil, fnerrors.InternalError("missing base server definition for %q", "distroless")
	}

	serverBase, err := pins.CheckImage(base.Base)
	if err != nil {
		return nil, err
	}

	return oci.ResolveImage(serverBase, target), nil
}

func ServerImageLLB(name string, target specs.Platform) (llb.State, error) {
	base := pins.Server(name)
	if base == nil || base.Base == "" {
		return llb.State{}, fnerrors.InternalError("missing base server definition for %q", "distroless")
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