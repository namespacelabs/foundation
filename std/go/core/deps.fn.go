// This file was automatically generated.
package core

import (
	"context"
)

type _checkProvideLivenessCheck func(context.Context, *LivenessCheck) (Check, error)

var _ _checkProvideLivenessCheck = ProvideLivenessCheck

type _checkProvideReadinessCheck func(context.Context, *ReadinessCheck) (Check, error)

var _ _checkProvideReadinessCheck = ProvideReadinessCheck
