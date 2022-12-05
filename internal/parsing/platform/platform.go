// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package platform

import (
	"github.com/containerd/containerd/platforms"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/schema"
)

func RuntimePlatform() specs.Platform {
	return platforms.DefaultSpec()
}

func PlatformToProto(s specs.Platform) *schema.Platform {
	return &schema.Platform{
		Os:           s.OS,
		Architecture: s.Architecture,
		Variant:      s.Variant,
	}
}

func ParsePlatform(str string) (specs.Platform, error) {
	return platforms.Parse(str)
}

func FormatPlatform(platform specs.Platform) string {
	return platforms.Format(platform)
}

func FormatPlatforms(platforms []specs.Platform) []string {
	strs := make([]string, len(platforms))
	for k, plat := range platforms {
		strs[k] = FormatPlatform(plat)
	}
	return strs
}

func ProtoToPlatform(p *schema.Platform) specs.Platform {
	return specs.Platform{
		Architecture: p.Architecture,
		OS:           p.Os,
		Variant:      p.Variant,
	}
}
