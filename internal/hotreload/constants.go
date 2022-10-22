// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package hotreload

import "namespacelabs.dev/foundation/schema"

var (
	ControllerPkg = schema.MakePackageSingleRef("namespacelabs.dev/foundation/std/development/filesync/controller")
)

const (
	FileSyncPort = 50000
)
