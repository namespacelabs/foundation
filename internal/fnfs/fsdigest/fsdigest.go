// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fsdigest

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
)

func Register() {
	compute.RegisterDigester[fs.FS](digester{})
}

type digester struct{}

func Compute(ctx context.Context, fsys fs.FS) (schema.Digest, error) {
	layer, err := oci.LayerFromFS(ctx, fsys)
	if err != nil {
		return schema.Digest{}, err
	}
	digest, err := layer.Digest()
	return schema.Digest(digest), err
}

func (digester) ComputeDigest(ctx context.Context, fsys fs.FS) (schema.Digest, error) {
	return Compute(ctx, fsys)
}
