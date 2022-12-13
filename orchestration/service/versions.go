// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/orchestration/server/constants"
)

const (
	backgroundUpdateInterval = 30 * time.Minute
	fetchLatestTimeout       = 2 * time.Second
)

type versionChecker struct {
	serverCtx context.Context

	planVersion int32

	mu             sync.Mutex
	requiresUpdate bool
}

func newVersionChecker(ctx context.Context) *versionChecker {
	version := os.Getenv("ORCH_VERSION")
	planVersion, err := strconv.ParseInt(version, 10, 32)
	if err != nil {
		log.Fatalf("unable to compute current version: %v", err)
	}

	vc := &versionChecker{
		serverCtx:   ctx,
		planVersion: int32(planVersion),
	}

	go func() {
		for {
			if err := vc.updateLatest(); err != nil {
				log.Printf("failed to update latest: %v\n", err)
			}

			time.Sleep(backgroundUpdateInterval)
		}
	}()

	return vc
}

func (vc *versionChecker) GetOrchestratorVersion(skipCache bool) (*proto.GetOrchestratorVersionResponse, error) {
	if skipCache {
		if err := vc.updateLatest(); err != nil {
			return nil, err
		}
	}

	vc.mu.Lock()
	defer vc.mu.Unlock()
	return &proto.GetOrchestratorVersionResponse{
		RequiresUpdate: vc.requiresUpdate,
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

		vc.requiresUpdate = plan.Version != vc.planVersion
	}

	return nil
}
