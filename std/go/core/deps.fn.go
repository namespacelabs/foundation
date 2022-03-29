// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This file was automatically generated.
package core

import (
	"context"
)

type _checkProvideLivenessCheck func(context.Context, string, *LivenessCheck) (Check, error)

var _ _checkProvideLivenessCheck = ProvideLivenessCheck

type _checkProvideReadinessCheck func(context.Context, string, *ReadinessCheck) (Check, error)

var _ _checkProvideReadinessCheck = ProvideReadinessCheck