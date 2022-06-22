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
