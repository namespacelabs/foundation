// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package build

import (
	"context"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrebuiltPlan(imgid oci.ImageID, platformIndependent bool, opts oci.ResolveOpts) Spec {
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
	opts                oci.ResolveOpts
}

func (p prebuilt) BuildImage(ctx context.Context, _ ops.Environment, conf Configuration) (compute.Computable[oci.Image], error) {
	return oci.ImageP(p.imgid.ImageRef(), conf.TargetPlatform(), p.opts), nil
}

func (p prebuilt) PlatformIndependent() bool { return p.platformIndependent }

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
