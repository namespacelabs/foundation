// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Canonical(ctx context.Context, img Image) (Image, error) {
	return tasks.Return(ctx, tasks.Action("oci.image.make-canonical"), func(ctx context.Context) (Image, error) {
		return mutate.Canonical(img)
	})
}
