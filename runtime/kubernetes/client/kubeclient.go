// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"context"
	"encoding/json"
	sync "sync"

	k8s "k8s.io/client-go/kubernetes"
	tadmissionregistrationv1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	tappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	tbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	tcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	tnetworkv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	trbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
)

var (
	clientCache struct {
		mu    sync.Mutex
		cache map[string]*k8s.Clientset
	}
)

func init() {
	clientCache.cache = map[string]*k8s.Clientset{}
}

func NewRestConfigFromHostEnv(ctx context.Context, host *HostConfig) (*restclient.Config, error) {
	cfg := host.HostEnv

	if cfg.GetIncluster() {
		return restclient.InClusterConfig()
	}

	if cfg.GetKubeconfig() == "" {
		return nil, fnerrors.New("hostEnv.Kubeconfig is required")
	}

	kubecfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.GetKubeconfig()},
		&clientcmd.ConfigOverrides{CurrentContext: cfg.GetContext()})

	computed, err := kubecfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	if err := computeBearerToken(ctx, host, computed); err != nil {
		return nil, err
	}

	return computed, nil
}

func NewClient(ctx context.Context, host *HostConfig) (*k8s.Clientset, error) {
	keyBytes, err := json.Marshal(struct {
		C *HostEnv
		E *schema.Environment
	}{host.HostEnv, host.Env})
	if err != nil {
		return nil, fnerrors.InternalError("failed to serialize config/env key: %w", err)
	}

	key := string(keyBytes)

	clientCache.mu.Lock()
	defer clientCache.mu.Unlock()

	if _, ok := clientCache.cache[key]; !ok {
		config, err := NewRestConfigFromHostEnv(ctx, host)
		if err != nil {
			return nil, err
		}

		clientset, err := k8s.NewForConfig(config)
		if err != nil {
			return nil, err
		}

		clientCache.cache[key] = clientset
	}

	return clientCache.cache[key], nil
}

func MakeResourceSpecificClient(resource string, cfg *restclient.Config) (restclient.Interface, error) {
	switch resource {
	case "configmaps", "secrets", "serviceaccounts", "pods", "services", "endpoints", "namespaces", "persistentvolumeclaims":
		c, err := tcorev1.NewForConfig(cfg)
		if err != nil {
			return nil, err
		}
		return c.RESTClient(), nil
	case "deployments", "statefulsets":
		c, err := tappsv1.NewForConfig(cfg)
		if err != nil {
			return nil, err
		}
		return c.RESTClient(), nil
	case "clusterroles", "clusterrolebindings", "roles", "rolebindings":
		c, err := trbacv1.NewForConfig(cfg)
		if err != nil {
			return nil, err
		}
		return c.RESTClient(), nil
	case "ingresses", "ingressclasses":
		c, err := tnetworkv1.NewForConfig(cfg)
		if err != nil {
			return nil, err
		}
		return c.RESTClient(), nil
	case "validatingwebhookconfigurations":
		c, err := tadmissionregistrationv1.NewForConfig(cfg)
		if err != nil {
			return nil, err
		}
		return c.RESTClient(), nil
	case "jobs":
		c, err := tbatchv1.NewForConfig(cfg)
		if err != nil {
			return nil, err
		}
		return c.RESTClient(), nil
	}

	return nil, fnerrors.InternalError("%s: don't know how to construct client", resource)
}

func ResolveConfig(ctx context.Context, env ops.Environment) (*restclient.Config, error) {
	if x, ok := env.(interface {
		KubeconfigProvider() (*HostConfig, error)
	}); ok {
		cfg, err := x.KubeconfigProvider()
		if err != nil {
			return nil, err
		}

		return NewRestConfigFromHostEnv(ctx, cfg)
	}

	cfg, err := ComputeHostConfig(env.Workspace(), env.DevHost(), env.Proto())
	if err != nil {
		return nil, err
	}

	return NewRestConfigFromHostEnv(ctx, cfg)
}

func ComputeHostConfig(ws *schema.Workspace, devHost *schema.DevHost, env *schema.Environment) (*HostConfig, error) {
	cfg := devhost.ConfigurationForEnvParts(devHost, env)

	hostEnv := &HostEnv{}
	if !cfg.Get(hostEnv) {
		return nil, fnerrors.UserError(nil, "%s: no kubernetes runtime configuration available", env.Name)
	}

	var err error
	hostEnv.Kubeconfig, err = dirs.ExpandHome(hostEnv.Kubeconfig)
	if err != nil {
		return nil, fnerrors.InternalError("failed to expand %q", hostEnv.Kubeconfig)
	}

	return &HostConfig{Workspace: ws, DevHost: devHost, Env: env, HostEnv: hostEnv}, nil
}
