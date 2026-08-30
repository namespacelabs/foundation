// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Forked from github.com/docker/buildx/util/platformutil/parse.go (v0.32.1).

package buildxstore

import (
	"strings"

	"github.com/containerd/platforms"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func parsePlatforms(platformsStr []string) ([]ocispecs.Platform, error) {
	if len(platformsStr) == 0 {
		return nil, nil
	}
	out := make([]ocispecs.Platform, 0, len(platformsStr))
	for _, s := range platformsStr {
		parts := strings.Split(s, ",")
		if len(parts) > 1 {
			p, err := parsePlatforms(parts)
			if err != nil {
				return nil, err
			}
			out = append(out, p...)
			continue
		}
		p, err := parsePlatform(s)
		if err != nil {
			return nil, err
		}
		out = append(out, platforms.Normalize(p))
	}
	return out, nil
}

func parsePlatform(in string) (ocispecs.Platform, error) {
	if strings.EqualFold(in, "local") {
		return platforms.DefaultSpec(), nil
	}
	return platforms.Parse(in)
}
