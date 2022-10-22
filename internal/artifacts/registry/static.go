// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package registry

import (
	"strings"

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

func (sr staticRegistry) AllocateName(repository string) compute.Computable[oci.AllocatedName] {
	return AllocateStaticName(sr, sr.r.Url, repository)
}

func AllocateStaticName(r Manager, url, repository string) compute.Computable[oci.AllocatedName] {
	if strings.HasSuffix(url, "/") {
		url += repository
	} else {
		url += "/" + repository
	}

	imgid := oci.ImageID{Repository: url}

	return StaticName(r, imgid, r.IsInsecure(), nil)
}

func (sr staticRegistry) AttachKeychain(img oci.ImageID) (oci.AllocatedName, error) {
	return oci.AllocatedName{
		Parent:           sr,
		InsecureRegistry: sr.r.GetInsecure(),
		ImageID:          img,
	}, nil
}
