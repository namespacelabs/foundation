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

	mu     sync.Mutex
	pinned []*schema.Workspace_BinaryDigest
}

func newVersionChecker(ctx context.Context) *versionChecker {
	current, err := getCurrentVersion(ctx)
	if err != nil {
		log.Fatalf("unable to compute current version: %v", err)
	}

	vc := &versionChecker{
		serverCtx: ctx,
		current:   current,
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
		Current: vc.current,
		Pinned:  vc.pinned,
	}, nil
}

func getCurrentVersion(ctx context.Context) (*schema.Workspace_BinaryDigest, error) {
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
	ctx, cancel := context.WithTimeout(vc.serverCtx, fetchLatestTimeout)
	defer cancel()

	res, err := fnapi.GetLatestPrebuilts(ctx, constants.ServerPkg, constants.ToolPkg)
	if err != nil {
		return fmt.Errorf("failed to fetch latest orch version from API server: %v", err)
	}

	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.pinned = nil
	for _, p := range res.Prebuilt {
		vc.pinned = append(vc.pinned, &schema.Workspace_BinaryDigest{
			PackageName: p.PackageName,
			Repository:  p.Repository,
			Digest:      p.Digest,
		})
	}

	return nil
}
