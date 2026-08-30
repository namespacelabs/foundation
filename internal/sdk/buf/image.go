// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buf

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/sdk/buf/image"
)

func State(ctx context.Context, target specs.Platform) (llb.State, error) {
	if binary.UsePrebuilts {
		return image.Prebuilt(ctx, target)
	}

	return image.ImagePlanWithNodeJS(target), nil
}
