// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gitpod

import "os"

func IsGitpod() bool {
	return os.Getenv("GITPOD_WORKSPACE_ID") != ""
}
