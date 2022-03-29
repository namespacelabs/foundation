// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package llbutil

import (
	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func Image(image string, platform specs.Platform) llb.State {
	return llb.Image(image, llb.ResolveModeDefault, llb.Platform(platform), ImageName(image, &platform))
}

func ImageName(image string, platform *specs.Platform) llb.ConstraintsOpt {
	if platform == nil {
		return llb.WithCustomNamef("Image: %s (host)", image)
	}
	return llb.WithCustomNamef("Image: %s (%s/%s)", image, platform.OS, platform.Architecture)
}