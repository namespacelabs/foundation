// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buf

import (
	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/sdk/buf/image"
)

func State(target specs.Platform) llb.State {
	if binary.UsePrebuilts {
		return image.Prebuilt(target)
	}

	return image.ImagePlanWithNodeJS(target)
}
