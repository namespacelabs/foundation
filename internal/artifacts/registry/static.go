// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package registry

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/compute"
)

type staticRegistry struct{ r *registry.Registry }

var _ Manager = staticRegistry{}

func MakeStaticRegistry(r *registry.Registry) Manager {
	return staticRegistry{r}
}

func (sr staticRegistry) Access() oci.RegistryAccess {
	return oci.RegistryAccess{
		InsecureRegistry: sr.r.Insecure,
		Keychain:         sr.keychain(),
		Transport:        sr.r.Transport,
	}
}

func (sr staticRegistry) AllocateName(repository string) compute.Computable[oci.RepositoryWithParent] {
	if sr.r.SingleRepository {
		return StaticRepository(sr, sr.r.Url, sr.Access())
	}

	return AllocateStaticName(sr, sr.r.Url, repository, sr.Access())
}

func AllocateStaticName(r Manager, url, repository string, access oci.RegistryAccess) compute.Computable[oci.RepositoryWithParent] {
	if strings.HasSuffix(url, "/") {
		url += repository
	} else {
		url += "/" + repository
	}

	return StaticRepository(r, url, access)
}

func (sr staticRegistry) keychain() oci.Keychain {
	if sr.r.UseDockerAuth {
		return defaultDockerKeychain{}
	}

	return nil
}

func AttachStaticKeychain(r Manager, repository string, access oci.RegistryAccess) oci.RepositoryWithParent {
	return oci.RepositoryWithParent{
		Parent: r,
		RepositoryWithAccess: oci.RepositoryWithAccess{
			RegistryAccess: access,
			Repository:     repository,
		},
	}
}

type defaultDockerKeychain struct{}

func (defaultDockerKeychain) Resolve(_ context.Context, res authn.Resource) (authn.Authenticator, error) {
	return authn.DefaultKeychain.Resolve(res)
}
