// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package core

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/std/core/types"
)

type Check struct {
	register func(string, Checker)
	owner    *InstantiationPath
}

func (c Check) Register(name string, check Checker) {
	c.register(fmt.Sprintf("%s %s", c.owner, name), check)
}

func ProvideLivenessCheck(ctx context.Context, _ *types.LivenessCheckArgs) (Check, error) {
	return Check{registerLiveness, InstantiationPathFromContext(ctx)}, nil
}

func ProvideReadinessCheck(ctx context.Context, _ *types.ReadinessCheckArgs) (Check, error) {
	return Check{registerReadiness, InstantiationPathFromContext(ctx)}, nil
}
