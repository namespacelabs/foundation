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

	"namespacelabs.dev/foundation/framework/runtime"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/orchestration/server/constants"
	"namespacelabs.dev/foundation/schema"
)

const (
	backgroundUpdateInterval = 30 * time.Minute
	fetchLatestTimeout       = 2 * time.Second
)

type versionChecker struct {
	serverCtx context.Context

	current *schema.Workspace_BinaryDigest

	mu       sync.Mutex
	pinned   []*schema.Workspace_BinaryDigest
	versions *proto.GetOrchestratorVersionResponse_Versions
	plan     *schema.DeployPlan
}

func newVersionChecker(ctx context.Context) *versionChecker {
	current, err := getCurrentDigest(ctx)
	if err != nil {
		log.Fatalf("unable to compute current version: %v", err)
	}

	version := os.Getenv("ORCH_VERSION")
	curr, err := strconv.ParseInt(version, 10, 32)
	if err != nil {
		log.Fatalf("unable to compute current version: %v", err)
	}

	vc := &versionChecker{
		serverCtx: ctx,
		current:   current,
		versions: &proto.GetOrchestratorVersionResponse_Versions{
			Current: int32(curr),
		},
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
		Current:  vc.current,
		Pinned:   vc.pinned,
		Versions: vc.versions,
		Plan:     vc.plan,
	}, nil
}

func getCurrentDigest(ctx context.Context) (*schema.Workspace_BinaryDigest, error) {
	cfg, err := runtime.LoadRuntimeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime config: %w", err)
	}

	parsed, err := oci.ParseImageID(cfg.Current.ImageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image ID: %w", err)
	}

	return &schema.Workspace_BinaryDigest{
		PackageName: constants.ServerPkg.String(),
		Repository:  parsed.Repository,
		Digest:      parsed.Digest,
	}, nil
}

func (vc *versionChecker) updateLatest() error {
	// Update prebuilts first, so that errors on the deployment plan path do not affect prebuilt updates.
	// TODO remove this path when we exclusively use deployment plans.
	if err := vc.updatePrebuilts(); err != nil {
		return err
	}

	return vc.updatePinnedPlan()
}

func (vc *versionChecker) updatePrebuilts() error {
	ctx, cancel := context.WithTimeout(vc.serverCtx, fetchLatestTimeout)
	defer cancel()

	prebuilts, err := fnapi.GetLatestPrebuilts(ctx, constants.ServerPkg, constants.ToolPkg)
	if err != nil {
		return fmt.Errorf("failed to fetch latest orch prebuilts from API server: %v", err)
	}

	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.pinned = nil
	for _, p := range prebuilts.Prebuilt {
		vc.pinned = append(vc.pinned, &schema.Workspace_BinaryDigest{
			PackageName: p.PackageName,
			Repository:  p.Repository,
			Digest:      p.Digest,
		})
	}

	return nil
}

func (vc *versionChecker) updatePinnedPlan() error {
	ctx, cancel := context.WithTimeout(vc.serverCtx, fetchLatestTimeout)
	defer cancel()

	plans, err := fnapi.GetLatestDeployPlans(ctx, constants.ServerPkg)
	if err != nil {
		return fmt.Errorf("failed to fetch latest orch deployment plan from API server: %v", err)
	}

	plan, version, err := parsePlan(plans)
	if err != nil {
		return err
	}

	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.versions.Latest = version
	vc.plan = plan

	return nil
}

func parsePlan(res *fnapi.GetLatestDeployPlansResponse) (*schema.DeployPlan, int32, error) {
	for _, plan := range res.Plan {
		if plan.PackageName != constants.ServerPkg.String() {
			continue
		}

		deployPlan := &schema.DeployPlan{}
		if err := plan.Plan.UnmarshalTo(deployPlan); err != nil {
			return nil, 0, fmt.Errorf("unable to unpack deployment plan: %w", err)
		}

		return deployPlan, plan.Version, nil
	}

	return nil, 0, fmt.Errorf("did not receive any deployment plan for orchestration server")
}
