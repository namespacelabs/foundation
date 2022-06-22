// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package handlers

import (
	"namespacelabs.dev/foundation/std/go/core"
)

var (
	ProvideDebugHandler   = core.ProvideDebugHandler
	ProvideLivenessCheck  = core.ProvideLivenessCheck
	ProvideReadinessCheck = core.ProvideReadinessCheck
	ProvideServerInfo     = core.ProvideServerInfo
)
