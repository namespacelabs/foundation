// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/workspace/dirs"
)

func NewDiscoveryClient(config *rest.Config, ephemeral bool) (discovery.CachedDiscoveryInterface, error) {
	cacheDir, err := makeCacheDir(config.Host, ephemeral)
	if err != nil {
		return nil, err
	}

	return diskcached.NewCachedDiscoveryClientForConfig(config, cacheDir, "", time.Duration(6*time.Hour))
}

func NewRESTMapper(config *rest.Config, ephemeral bool) (meta.RESTMapper, error) {
	discoveryClient, err := NewDiscoveryClient(config, ephemeral)
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

func makeCacheDir(host string, ephemeral bool) (string, error) {
	if ephemeral {
		return os.MkdirTemp(os.TempDir(), "kubernetes-discovery")
	}

	cacheDir, err := dirs.Ensure(dirs.Subdir("kubernetes"))
	if err != nil {
		return "", err
	}

	hostID := naming.StableID(host)

	discoveryCacheDir := filepath.Join(cacheDir, "discovery", hostID)
	return discoveryCacheDir, nil
}
