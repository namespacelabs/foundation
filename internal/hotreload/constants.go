// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package hotreload

import "namespacelabs.dev/foundation/schema"

var (
	ControllerPkg = schema.MakePackageSingleRef("namespacelabs.dev/foundation/std/development/filesync/controller")
)

const (
	FileSyncPort = 50000
)
