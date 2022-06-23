// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package unprepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type k3dUnprepare struct {
	k3dbin k3d.K3D
}

func UnprepareK3d(ctx context.Context) error {
	return tasks.Action("unprepare.k3d").Run(ctx, func(ctx context.Context) error {
		k3dbin, err := k3d.EnsureSDK(ctx)
		if err != nil {
			return err
		}

		u := &k3dUnprepare{k3dbin: k3dbin}

		if err := u.deleteClusters(ctx); err != nil {
			return err
		}

		if err := u.deleteRegistries(ctx); err != nil {
			return err
		}
		return nil
	})
}

func (u *k3dUnprepare) deleteRegistries(ctx context.Context) error {
	// TODO: be more selective and delete only registries in the devhost config.
	registries, err := u.k3dbin.ListRegistries(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, registry := range registries {
		if err := u.k3dbin.DeleteRegistry(ctx, registry.Name); err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.New(errs...)
}

func (u *k3dUnprepare) deleteClusters(ctx context.Context) error {
	// TODO: be more selective and delete clusters owned by Namespace.
	clusters, err := u.k3dbin.ListClusters(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, cluster := range clusters {
		if err := u.k3dbin.DeleteCluster(ctx, cluster.Name); err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.New(errs...)
}
