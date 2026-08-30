// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"fmt"
	"os"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type StampImage struct {
	Src build.Spec
}

func (m StampImage) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	anns := map[string]string{}

	if bkBuildURL := os.Getenv("BUILDKITE_BUILD_URL"); bkBuildURL != "" {
		bkJobID := os.Getenv("BUILDKITE_JOB_ID")
		anns["org.opencontainers.image.url"] = fmt.Sprintf("%s#%s", bkBuildURL, bkJobID)
	}

	if bkCommit := os.Getenv("BUILDKITE_COMMIT"); bkCommit != "" {
		anns["org.opencontainers.image.revision"] = bkCommit
	}

	im, err := m.Src.BuildImage(ctx, env, conf)
	if err != nil {
		return nil, err
	}

	named := oci.MakeNamedImage(m.Src.Description(), im)
	return oci.AnnotateImage(named, anns), nil
}

func (m StampImage) PlatformIndependent() bool { return m.Src.PlatformIndependent() }

func (m StampImage) Description() string { return fmt.Sprintf("Stamp(%s)", m.Src.Description()) }
