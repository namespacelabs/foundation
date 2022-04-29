// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func (r k8sRuntime) TargetPlatforms(ctx context.Context) ([]specs.Platform, error) {
	if r.env.Purpose == schema.Environment_PRODUCTION {
		// XXX make this configurable.
		return parsePlatforms([]string{"linux/amd64", "linux/arm64"})
	}

	raw, err := r.systemInfo.Wait(ctx)
	if err != nil {
		return nil, err
	}

	return parsePlatforms(raw.Value.(systemInfo).nodePlatforms)
}

func parsePlatforms(plats []string) ([]specs.Platform, error) {
	var ret []specs.Platform
	for _, p := range plats {
		parsed, err := devhost.ParsePlatform(p)
		if err != nil {
			return nil, err
		}
		ret = append(ret, parsed)
	}
	return ret, nil
}
