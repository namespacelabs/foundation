// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/orchestration/server/constants"
)

const (
	updateInterval     = 12 * time.Hour
	fetchLatestTimeout = 2 * time.Second
)

type versionChecker struct {
	serverCtx context.Context

	current int32

	mu        sync.Mutex
	latest    int32
	fetchedAt time.Time
}

func newVersionChecker(ctx context.Context) *versionChecker {
	vc := &versionChecker{
		serverCtx: ctx,
		current:   orchestration.ExecuteOpts().OrchestratorVersion,
	}

	if err := vc.updateLatest(); err != nil {
		// Will retry on GetOrchestratorVersion calls.
		log.Printf("failed to fetch latest version: %v\n", err)
	}

	return vc
}

func (vc *versionChecker) GetOrchestratorVersion(skipCache bool) (*proto.GetOrchestratorVersionResponse, error) {
	vc.mu.Lock()
	runUpdate := skipCache || time.Since(vc.fetchedAt) > updateInterval
	vc.mu.Unlock()

	if runUpdate {
		if err := vc.updateLatest(); err != nil {
			return nil, err
		}
	}

	vc.mu.Lock()
	defer vc.mu.Unlock()
	return &proto.GetOrchestratorVersionResponse{
		Current: vc.current,
		Latest:  vc.latest,
	}, nil
}

func (vc *versionChecker) updateLatest() error {
	ctx, cancel := context.WithTimeout(vc.serverCtx, fetchLatestTimeout)
	defer cancel()

	plans, err := fnapi.GetLatestDeployPlans(ctx, constants.ServerPkg)
	if err != nil {
		return fmt.Errorf("failed to fetch latest orch plan from API server: %v", err)
	}

	vc.mu.Lock()
	defer vc.mu.Unlock()

	for _, plan := range plans.Plan {
		if plan.PackageName != constants.ServerPkg.String() {
			continue
		}

		vc.latest = plan.Version
	}

	vc.fetchedAt = time.Now()

	return nil
}
