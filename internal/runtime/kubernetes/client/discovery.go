// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package client

import (
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/discovery"
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/framework/kubernetes/kubenaming"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func NewDiscoveryClient(config *rest.Config, ephemeral bool) (discovery.CachedDiscoveryInterface, error) {
	cacheDir, err := makeCacheDir(config.Host, ephemeral)
	if err != nil {
		return nil, err
	}

	return diskcached.NewCachedDiscoveryClientForConfig(config, cacheDir, "", time.Duration(6*time.Hour))
}

func makeCacheDir(host string, ephemeral bool) (string, error) {
	if ephemeral {
		return os.MkdirTemp(os.TempDir(), "kubernetes-discovery")
	}

	cacheDir, err := dirs.Ensure(dirs.Subdir("kubernetes"))
	if err != nil {
		return "", err
	}

	hostID := kubenaming.StableID(host)

	discoveryCacheDir := filepath.Join(cacheDir, "discovery", hostID)
	return discoveryCacheDir, nil
}
