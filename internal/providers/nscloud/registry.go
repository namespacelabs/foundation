// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nscloud

import (
	"context"
	"strings"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

var DefaultKeychain oci.Keychain = api.DefaultKeychain

type nscloudRegistry struct {
	clusterID string
	registry  *api.ImageRegistry
}

func RegisterRegistry() {
	registry.Register("nscloud", func(ctx context.Context, ck cfg.Configuration) (registry.Manager, error) {
		conf, ok := clusterConfigType.CheckGet(ck)
		if !ok || conf.ClusterId == "" {
			return nil, fnerrors.InternalError("missing registry configuration")
		}

		return nscloudRegistry{clusterID: conf.ClusterId}, nil
	})
}

func (r nscloudRegistry) Access() oci.RegistryAccess {
	return oci.RegistryAccess{
		InsecureRegistry: false,
		Keychain:         DefaultKeychain,
	}
}

func (r nscloudRegistry) AllocateName(repository string) compute.Computable[oci.RepositoryWithParent] {
	return compute.Map(tasks.Action("nscloud.allocate-repository").Arg("repository", repository),
		compute.Inputs().Str("repository", repository).Str("clusterID", r.clusterID),
		compute.Output{},
		func(ctx context.Context, _ compute.Resolved) (oci.RepositoryWithParent, error) {
			registry, err := r.fetchRegistry(ctx)
			if err != nil {
				return oci.RepositoryWithParent{}, err
			}

			url := registry.EndpointAddress
			if url == "" {
				return oci.RepositoryWithParent{}, fnerrors.InternalError("%s: cluster is missing registry", r.clusterID)
			}

			if registry.Repository != "" {
				if strings.HasSuffix(url, "/") {
					url += registry.Repository
				} else {
					url += "/" + registry.Repository
				}
			}

			if strings.HasSuffix(url, "/") {
				url += repository
			} else {
				url += "/" + repository
			}

			return oci.RepositoryWithParent{
				Parent: r,
				RepositoryWithAccess: oci.RepositoryWithAccess{
					Repository:     url,
					RegistryAccess: r.Access(),
				},
			}, nil
		})
}

func (r nscloudRegistry) fetchRegistry(ctx context.Context) (*api.ImageRegistry, error) {
	if r.registry != nil {
		return r.registry, nil
	}

	resp, err := api.GetCluster(ctx, api.Endpoint, r.clusterID)
	if err != nil {
		return nil, err
	}

	return resp.Registry, nil
}
