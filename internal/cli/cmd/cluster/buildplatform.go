// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type buildPlatform string

func parseBuildPlatform(value string) (buildPlatform, error) {
	switch strings.ToLower(value) {
	case "amd64":
		return "amd64", nil

	case "arm64":
		return "arm64", nil
	}

	return "", fnerrors.New("invalid build platform %q", value)
}
