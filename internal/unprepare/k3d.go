// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package unprepare

import (
	"context"

	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/std/tasks"
)

func UnprepareK3d(ctx context.Context) error {
	return tasks.Action("unprepare.k3d").Run(ctx, func(ctx context.Context) error {
		k3dbin, err := k3d.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return err
		}

		if err := deleteClusters(ctx, k3dbin, "ns"); err != nil {
			return err
		}

		if err := deleteRegistries(ctx, k3dbin, "k3d-ns-registry.nslocal.host"); err != nil {
			return err
		}

		return nil
	})
}

func deleteRegistries(ctx context.Context, k3dbin k3d.K3D, names ...string) error {
	registries, err := k3dbin.ListRegistries(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, registry := range registries {
		if !slices.Contains(names, registry.Name) {
			continue
		}

		if err := k3dbin.DeleteRegistry(ctx, registry.Name); err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.New(errs...)
}

func deleteClusters(ctx context.Context, k3dbin k3d.K3D, names ...string) error {
	clusters, err := k3dbin.ListClusters(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, cluster := range clusters {
		if !slices.Contains(names, cluster.Name) {
			continue
		}
		if err := k3dbin.DeleteCluster(ctx, cluster.Name); err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.New(errs...)
}
