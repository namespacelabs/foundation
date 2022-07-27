// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package registry

import (
	"strings"

	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/workspace/compute"
)

type staticRegistry struct{ r *registry.Registry }

var _ Manager = staticRegistry{}

func (sr staticRegistry) IsInsecure() bool {
	return sr.r.Insecure
}

func (sr staticRegistry) AllocateName(repository string) compute.Computable[oci.AllocatedName] {
	w := sr.r.Url

	if strings.HasSuffix(w, "/") {
		w += repository
	} else {
		w += "/" + repository
	}

	imgid := oci.ImageID{Repository: w}

	return StaticName(sr.r, imgid, nil)
}

func (sr staticRegistry) AuthRepository(img oci.ImageID) (oci.AllocatedName, error) {
	return oci.AllocatedName{
		InsecureRegistry: sr.r.GetInsecure(),
		ImageID:          img,
	}, nil
}
