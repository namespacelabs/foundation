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

func (sr staticRegistry) IsInsecure() bool {
	return sr.r.Insecure
}

func (sr staticRegistry) AllocateName(repository string) compute.Computable[oci.AllocatedRepository] {
	return AllocateStaticName(sr, sr.r.Url, repository, sr.keychain())
}

func AllocateStaticName(r Manager, url, repository string, keychain oci.Keychain) compute.Computable[oci.AllocatedRepository] {
	if strings.HasSuffix(url, "/") {
		url += repository
	} else {
		url += "/" + repository
	}

	imgid := oci.ImageID{Repository: url}

	return StaticName(r, imgid, r.IsInsecure(), keychain)
}

func (sr staticRegistry) AttachKeychain(img oci.ImageID) (oci.AllocatedRepository, error) {
	return AttachStaticKeychain(sr, img, sr.keychain()), nil
}

func (sr staticRegistry) keychain() oci.Keychain {
	if sr.r.UseDockerAuth {
		return defaultDockerKeychain{}
	}

	return nil
}

func AttachStaticKeychain(r Manager, img oci.ImageID, keychain oci.Keychain) oci.AllocatedRepository {
	return oci.AllocatedRepository{
		Parent: r,
		TargetRepository: oci.TargetRepository{
			InsecureRegistry: r.IsInsecure(),
			ImageID:          img,
			Keychain:         keychain,
		},
	}
}

type defaultDockerKeychain struct{}

func (defaultDockerKeychain) Resolve(_ context.Context, res authn.Resource) (authn.Authenticator, error) {
	return authn.DefaultKeychain.Resolve(res)
}
