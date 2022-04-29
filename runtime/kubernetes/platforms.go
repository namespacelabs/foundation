// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func (r k8sRuntime) TargetPlatforms() []specs.Platform {
	if r.env.Purpose == schema.Environment_PRODUCTION {
		// XXX make this configurable.
		plats := []string{"linux/amd64", "linux/arm64"}

		var ret []specs.Platform
		for _, p := range plats {
			parsed, err := devhost.ParsePlatform(p)
			if err != nil {
				panic(err)
			}
			ret = append(ret, parsed)
		}
		return ret
	}

	p := devhost.RuntimePlatform()
	p.OS = "linux" // We always run on linux.
	return []specs.Platform{p}
}
