// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/baseimage"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var baseImageRef = schema.MakePackageRef("namespacelabs.dev/foundation/library/golang/baseimage", "baseimage")

func baseProdImage(ctx context.Context, env pkggraph.SealedContext, plat specs.Platform) (oci.NamedImage, error) {
	return baseimage.Load(ctx, env, baseImageRef, plat)
}
