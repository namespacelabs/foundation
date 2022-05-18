// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
)

const k8sDriver = "namespacelabs.dev/foundation/std/runtime/kubernetes/driver"

func (r k8sRuntime) prepareDriver(ctx context.Context, env provision.ServerEnv) (*schema.Definition, error) {
	pkg, err := env.LoadByName(ctx, k8sDriver)
	if err != nil {
		return nil, err
	}

	platforms, err := r.TargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	prepared, err := binary.Plan(ctx, pkg, binary.BuildImageOpts{
		Platforms: platforms,
	})
	if err != nil {
		return nil, err
	}

	bid := provision.NewBuildID()
	tag, err := registry.AllocateName(ctx, env, k8sDriver, bid)
	if err != nil {
		return nil, err
	}

	bin, err := multiplatform.PrepareMultiPlatformImage(ctx, env, prepared.Plan)
	if err != nil {
		return nil, err
	}

	img := oci.PublishResolvable(tag, bin)
	v, err := compute.Get(ctx, img)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("%s-driver", r.env.Name)

	x, err := anypb.New(&kubedef.OpRun{
		Namespace: r.moduleNamespace,
		Name:      name,
		Image:     v.Value.RepoAndDigest(),
		Command:   prepared.Command,
	})
	if err != nil {
		return nil, err
	}

	return &schema.Definition{
		Description: "k8s driver",
		Impl:        x,
		Scope:       []string{k8sDriver},
	}, nil
}
