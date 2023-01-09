// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package registry

import (
	"context"
	"fmt"
	"strings"

	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/gcp/gke"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func Register() {
	registry.Register("gcp/artifactregistry", func(ctx context.Context, ck cfg.Configuration) (registry.Manager, error) {
		cluster, err := gke.ConfiguredCluster(ctx, ck)
		if err != nil {
			return nil, err
		}

		return manager{cluster}, nil
	})

	oci.RegisterDomainKeychain("pkg.dev", DefaultKeychain, oci.Keychain_UseOnWrites)
}

type manager struct {
	cluster *gke.Cluster
}

func (m manager) Access() oci.RegistryAccess {
	return oci.RegistryAccess{
		InsecureRegistry: false,
		Keychain:         keychain{m.cluster.TokenSource},
	}
}

func (m manager) AllocateName(repository string) compute.Computable[oci.RepositoryWithParent] {
	return compute.Inline(tasks.Action("artifactregistry.allocate-repository").Arg("repository", repository),
		func(ctx context.Context) (oci.RepositoryWithParent, error) {
			client, err := artifactregistry.NewClient(ctx, option.WithTokenSource(m.cluster.TokenSource))
			if err != nil {
				return oci.RepositoryWithParent{}, err
			}

			const repoID = "namespace-managed"
			location := region(m.cluster.Cluster.Location)
			op, err := client.CreateRepository(ctx, &artifactregistrypb.CreateRepositoryRequest{
				Parent:       fmt.Sprintf("projects/%s/locations/%s", m.cluster.ProjectID, location),
				RepositoryId: repoID,
				Repository: &artifactregistrypb.Repository{
					Format: artifactregistrypb.Repository_DOCKER,
				},
			})
			if err != nil {
				if status.Code(err) != codes.AlreadyExists {
					return oci.RepositoryWithParent{}, fnerrors.InvocationError("gcp/artifactregistry", "failed to construct request: %w", err)
				}
			} else if err == nil {
				if _, err := op.Wait(ctx); err != nil {
					return oci.RepositoryWithParent{}, fnerrors.InvocationError("gcp/artifactregistry", "failed: %w", err)
				}
			}

			repo := oci.RepositoryWithAccess{
				RegistryAccess: m.Access(),
				Repository:     fmt.Sprintf("%s-docker.pkg.dev/%s/%s/%s", location, m.cluster.ProjectID, repoID, repository),
			}

			return oci.RepositoryWithParent{
				Parent:               m,
				RepositoryWithAccess: repo,
			}, nil
		})
}

func region(loc string) string {
	parts := strings.Split(loc, "-")
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, "-")
}
