// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package planning

import (
	"context"

	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/framework/secrets/localsecrets"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/std/cfg"
)

type Planner struct {
	Context  cfg.Context
	Runtime  runtime.Planner
	Registry registry.Manager
	Secrets  secrets.SecretsSource
}

func NewPlanner(ctx context.Context, env cfg.Context) (Planner, error) {
	planner, err := runtime.PlannerFor(ctx, env)
	if err != nil {
		return Planner{}, err
	}

	source, err := localsecrets.NewLocalSecrets(env)
	if err != nil {
		return Planner{}, err
	}

	return Planner{
		Context:  env,
		Runtime:  planner,
		Registry: planner.Registry(),
		Secrets:  source,
	}, nil
}

func NewPlannerFromExisting(env cfg.Context, planner runtime.Planner) (Planner, error) {
	source, err := localsecrets.NewLocalSecrets(env)
	if err != nil {
		return Planner{}, err
	}

	return Planner{
		Context:  env,
		Runtime:  planner,
		Registry: planner.Registry(),
		Secrets:  source,
	}, nil
}
