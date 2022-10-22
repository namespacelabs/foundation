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

// By default, a Readiness checker never returns a failure after it becomes
// valid once. This is by design, we don't want our server to stop receiving
// traffic in the event of intermittent issues (e.g. we don't want to track
// whether our dependencies are reachable). To manually control the behavior
// pass in a `ManualChecker` instead.
func (c Check) Register(name string, check Checker) {
	c.register(fmt.Sprintf("%s %s", c.owner, name), check)
}

func (c Check) RegisterFunc(name string, check CheckerFunc) {
	c.Register(name, check)
}

func ManualChecker(check CheckerFunc) Checker {
	return manualChecker{check}
}

func ProvideLivenessCheck(ctx context.Context, _ *types.LivenessCheckArgs) (Check, error) {
	return Check{registerLiveness, InstantiationPathFromContext(ctx)}, nil
}

func ProvideReadinessCheck(ctx context.Context, _ *types.ReadinessCheckArgs) (Check, error) {
	return Check{registerReadiness, InstantiationPathFromContext(ctx)}, nil
}

type manualChecker struct{ c CheckerFunc }

func (m manualChecker) Check(ctx context.Context) error { return m.c(ctx) }
func (m manualChecker) isManual() bool                  { return true }
