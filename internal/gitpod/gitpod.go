// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package gitpod

import "os"

func IsGitpod() bool {
	return os.Getenv("GITPOD_WORKSPACE_ID") != ""
}
