// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package build

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func PrebuiltResolveOpts() oci.RegistryAccess {
	// We assume all prebuilts are public, unless noted otherwise.
	return oci.RegistryAccess{
		PublicImage: true,
	}
}

func PrebuiltPlan(imgid oci.ImageID, platformIndependent bool, opts oci.RegistryAccess) Spec {
	return prebuilt{imgid, platformIndependent, opts}
}

func IsPrebuilt(spec Spec) (oci.ImageID, bool) {
	if x, ok := spec.(prebuilt); ok {
		return x.imgid, true
	}
	return oci.ImageID{}, false
}

type prebuilt struct {
	imgid               oci.ImageID
	platformIndependent bool
	opts                oci.RegistryAccess
}

func (p prebuilt) BuildImage(ctx context.Context, _ pkggraph.SealedContext, conf Configuration) (compute.Computable[oci.Image], error) {
	return oci.ImageP(p.imgid.ImageRef(), conf.TargetPlatform(), p.opts), nil
}

func (p prebuilt) PlatformIndependent() bool { return p.platformIndependent }

func (p prebuilt) Description() string { return fmt.Sprintf("prebuilt(%s)", p.imgid) }

func Prebuilt(imgid oci.ImageID) compute.Computable[oci.ImageID] {
	return &prebuiltImage{imgid: imgid}
}

type prebuiltImage struct {
	imgid oci.ImageID

	compute.PrecomputeScoped[oci.ImageID]
}

var _ compute.Digestible = prebuiltImage{}

func (p prebuiltImage) Action() *tasks.ActionEvent {
	return tasks.Action("build.plan.prebuilt").Arg("ref", p.imgid)
}
func (p prebuiltImage) Inputs() *compute.In { return compute.Inputs().Stringer("ref", p.imgid) }
func (p prebuiltImage) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}
func (p prebuiltImage) Compute(context.Context, compute.Resolved) (oci.ImageID, error) {
	return p.imgid, nil
}

func (p prebuiltImage) ComputeDigest(context.Context) (schema.Digest, error) {
	return schema.DigestOf(p.imgid)
}
