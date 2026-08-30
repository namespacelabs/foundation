// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package process

import "os/exec"

func SetSIDAttr(cmd *exec.Cmd, val bool) {
	// Means nothing on Windows.
}

func ForegroundAttr(cmd *exec.Cmd, val bool) {
	// Means nothing on Windows.
}
