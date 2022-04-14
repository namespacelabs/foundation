// This file was automatically generated.
package core

import (
	"context"
)

type _checkProvideDebugHandler func(context.Context, *DebugHandlerArgs) (DebugHandler, error)

var _ _checkProvideDebugHandler = ProvideDebugHandler

type _checkProvideLivenessCheck func(context.Context, *LivenessCheckArgs) (Check, error)

var _ _checkProvideLivenessCheck = ProvideLivenessCheck

type _checkProvideReadinessCheck func(context.Context, *ReadinessCheckArgs) (Check, error)

var _ _checkProvideReadinessCheck = ProvideReadinessCheck
