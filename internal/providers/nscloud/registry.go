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

var DefaultKeychain = api.DefaultKeychain

type nscloudRegistry struct {
	registry *api.ImageRegistry
}

func RegisterRegistry() {
	registry.Register("nscloud", func(ctx context.Context, ck cfg.Configuration) (registry.Manager, error) {
		return nscloudRegistry{}, nil
	})
}

func (r nscloudRegistry) Access() oci.RegistryAccess {
	return oci.RegistryAccess{
		InsecureRegistry: false,
		Keychain:         DefaultKeychain,
	}
}

func (r nscloudRegistry) AllocateName(repository, tag string) compute.Computable[oci.RepositoryWithParent] {
	return compute.Map(tasks.Action("nscloud.allocate-repository").Arg("repository", repository),
		compute.Inputs().Str("repository", repository),
		compute.Output{},
		func(ctx context.Context, _ compute.Resolved) (oci.RepositoryWithParent, error) {
			registry, err := r.fetchRegistry(ctx)
			if err != nil {
				return oci.RepositoryWithParent{}, err
			}

			url := registry.EndpointAddress
			if url == "" {
				return oci.RepositoryWithParent{}, fnerrors.InternalError("registry is missing endpoint")
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
					UserTag:        tag,
					RegistryAccess: r.Access(),
				},
			}, nil
		})
}

func (r nscloudRegistry) fetchRegistry(ctx context.Context) (*api.ImageRegistry, error) {
	if r.registry != nil {
		return r.registry, nil
	}

	rs, err := api.GetImageRegistry(ctx, api.Methods)
	if err != nil {
		return nil, err
	}

	if rs.NSCR == nil {
		return nil, fnerrors.InternalError("expected nscr to be in response")
	}

	return rs.NSCR, nil
}
