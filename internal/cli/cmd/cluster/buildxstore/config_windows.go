// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Forked from github.com/docker/buildx/util/confutil/config_windows.go (v0.32.1).

package buildxstore

import "os"

func sudoer(_ string) *chowner {
	return nil
}

func fileOwner(_ os.FileInfo) *chowner {
	return nil
}
