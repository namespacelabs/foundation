// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/parsing/platform"
)

var (
	UseNodePlatformsForProduction = false
	ProductionPlatforms           = []string{"linux/amd64", "linux/arm64"}
)

func (r *Cluster) SystemInfo(ctx context.Context) (*kubedef.SystemInfo, error) {
	return r.FetchSystemInfo(ctx)
}

func (r *Cluster) UnmatchedTargetPlatforms(ctx context.Context) ([]specs.Platform, error) {
	sysInfo, err := r.FetchSystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	return parsePlatforms(sysInfo.NodePlatform)
}

func parsePlatforms(plats []string) ([]specs.Platform, error) {
	var ret []specs.Platform
	for _, p := range plats {
		parsed, err := platform.ParsePlatform(p)
		if err != nil {
			return nil, err
		}
		ret = append(ret, parsed)
	}
	return ret, nil
}
