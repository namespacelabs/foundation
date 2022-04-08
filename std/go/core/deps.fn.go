// This file was automatically generated.
package core

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
)

type _checkProvideLivenessCheck func(context.Context, fninit.Caller, *LivenessCheck) (Check, error)

var _ _checkProvideLivenessCheck = ProvideLivenessCheck

type _checkProvideReadinessCheck func(context.Context, fninit.Caller, *ReadinessCheck) (Check, error)

var _ _checkProvideReadinessCheck = ProvideReadinessCheck
