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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/schema"
)

const (
	serverId                      = "0fomj22adbua2u0ug3og"
	serverPkg  schema.PackageName = "namespacelabs.dev/foundation/orchestration/server"
	toolPkg    schema.PackageName = "namespacelabs.dev/foundation/orchestration/server/tool"
	serverName                    = "orchestration-api-server"

	backgroundUpdateInterval = 30 * time.Minute
	fetchLatestTimeout       = 30 * time.Second // can be generous, since we don't block in this.
	cacheTimeout             = time.Minute
)

type versionChecker struct {
	serverCtx context.Context

	current *schema.Workspace_BinaryDigest

	mu        sync.Mutex
	pinned    []*schema.Workspace_BinaryDigest
	fetchedAt time.Time
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
			vc.updateLatest()

			time.Sleep(backgroundUpdateInterval)
		}
	}()

	return vc
}

func (vc *versionChecker) GetOrchestratorVersion() *proto.GetOrchestratorVersionResponse {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	// shedule a non-blocking update so that future calls get the latest version.
	go vc.updateLatest()

	return &proto.GetOrchestratorVersionResponse{
		Current: vc.current,
		Pinned:  vc.pinned,
	}
}

func getCurrentVersion(ctx context.Context) (*schema.Workspace_BinaryDigest, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create incluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create incluster clientset: %w", err)
	}

	pods, err := clientset.CoreV1().Pods(kubedef.AdminNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: kubedef.SerializeSelector(map[string]string{
			kubedef.K8sServerId: serverId,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		for _, ctr := range pod.Spec.Containers {
			if ctr.Name == serverName {
				parsed, err := oci.ParseImageID(ctr.Image)
				if err != nil {
					return nil, fmt.Errorf("failed to parse image ID: %w", err)
				}

				return &schema.Workspace_BinaryDigest{
					PackageName: serverPkg.String(),
					Repository:  parsed.Repository,
					Digest:      parsed.Digest,
				}, nil
			}
		}

		return nil, fmt.Errorf("did not find main container in orchestrator pod %s", pod.Name)
	}

	return nil, fmt.Errorf("did not find any running orchestrator pod")
}

func (vc *versionChecker) updateLatest() {
	if !vc.shouldUpdate() {
		return
	}

	ctx, cancel := context.WithTimeout(vc.serverCtx, fetchLatestTimeout)
	defer cancel()

	res, err := fnapi.GetLatestPrebuilts(ctx, serverPkg, toolPkg)
	if err != nil {
		log.Printf("failed to fetch latest orch version from API server: %v\n", err)
		return
	}

	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.fetchedAt = time.Now()
	vc.pinned = nil
	for _, p := range res.Prebuilt {
		vc.pinned = append(vc.pinned, &schema.Workspace_BinaryDigest{
			PackageName: p.PackageName,
			Repository:  p.Repository,
			Digest:      p.Digest,
		})
	}
}

func (vc *versionChecker) shouldUpdate() bool {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	return time.Since(vc.fetchedAt) > cacheTimeout
}
