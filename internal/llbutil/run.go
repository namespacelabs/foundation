// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package llbutil

import (
	"fmt"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

var GitCredentialsBuildkitSecret string

func PrefixSh(prefix string, platform *specs.Platform, command string, args ...interface{}) []llb.RunOption {
	if platform != nil {
		prefix += fmt.Sprintf(" %s/%s", platform.OS, platform.Architecture)
	}

	return []llb.RunOption{
		llb.Shlexf(command, args...),
		llb.WithCustomNamef("[%s] %s", prefix, fmt.Sprintf(command, args...)),
	}
}
