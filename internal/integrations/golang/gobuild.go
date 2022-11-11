// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"golang.org/x/mod/semver"
)

func goBuildArgs(goVersion string) []string {
	args := []string{"build", "-v", "-trimpath"}

	// VCS information is not included in the binaries, to ensure we have reproducible builds.
	if semver.Compare("v"+goVersion, "v1.18") >= 0 {
		args = append(args, "-buildvcs=false")
	}

	return args
}
