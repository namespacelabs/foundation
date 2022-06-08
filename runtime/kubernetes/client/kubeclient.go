// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"context"
	"encoding/json"
	sync "sync"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	fnschema "namespacelabs.dev/foundation/schema"
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

func NewRestConfigFromHostEnv(ctx context.Context, host *HostConfig) (*rest.Config, error) {
	cfg := host.HostEnv

	if cfg.GetIncluster() {
		return rest.InClusterConfig()
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
		S string
	}{host.HostEnv, host.Selector.HashKey()})
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

var groups = map[string]schema.GroupVersion{
	"configmaps":             corev1.SchemeGroupVersion,
	"secrets":                corev1.SchemeGroupVersion,
	"serviceaccounts":        corev1.SchemeGroupVersion,
	"pods":                   corev1.SchemeGroupVersion,
	"services":               corev1.SchemeGroupVersion,
	"endpoints":              corev1.SchemeGroupVersion,
	"namespaces":             corev1.SchemeGroupVersion,
	"persistentvolumeclaims": corev1.SchemeGroupVersion,

	"deployments":  appsv1.SchemeGroupVersion,
	"statefulsets": appsv1.SchemeGroupVersion,

	"clusterroles":        rbacv1.SchemeGroupVersion,
	"clusterrolebindings": rbacv1.SchemeGroupVersion,
	"roles":               rbacv1.SchemeGroupVersion,
	"rolebindings":        rbacv1.SchemeGroupVersion,

	"ingresses":      networkingv1.SchemeGroupVersion,
	"ingressclasses": networkingv1.SchemeGroupVersion,

	"validatingwebhookconfigurations": admissionregistrationv1.SchemeGroupVersion,

	"jobs": batchv1.SchemeGroupVersion,

	"customresourcedefinitions": apiextensionsv1.SchemeGroupVersion,
}

func MakeResourceSpecificClient(resource string, cfg *rest.Config) (rest.Interface, error) {
	gv, ok := groups[resource]
	if !ok {
		return nil, fnerrors.InternalError("%s: don't know how to construct a client for this resource", resource)
	}

	return newForConfigAndGroupVersion(cfg, gv)
}

func setConfigDefaults(config *rest.Config, gv schema.GroupVersion) error {
	config.GroupVersion = &gv
	config.APIPath = "/api"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

func newForConfigAndGroupVersion(c *rest.Config, gv schema.GroupVersion) (*rest.RESTClient, error) {
	config := *c
	if err := setConfigDefaults(&config, gv); err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(&config)
	if err != nil {
		return nil, err
	}
	return rest.RESTClientForConfigAndClient(&config, httpClient)
}

func ResolveConfig(ctx context.Context, env ops.Environment) (*rest.Config, error) {
	if x, ok := env.(interface {
		KubeconfigProvider() (*HostConfig, error)
	}); ok {
		cfg, err := x.KubeconfigProvider()
		if err != nil {
			return nil, err
		}

		return NewRestConfigFromHostEnv(ctx, cfg)
	}

	cfg, err := ComputeHostConfig(env.DevHost(), devhost.ByEnvironment(env.Proto()))
	if err != nil {
		return nil, err
	}

	return NewRestConfigFromHostEnv(ctx, cfg)
}

func ComputeHostConfig(devHost *fnschema.DevHost, selector devhost.Selector) (*HostConfig, error) {
	cfg := devhost.Select(devHost, selector)

	hostEnv := &HostEnv{}
	if !cfg.Get(hostEnv) {
		return nil, fnerrors.UserError(nil, "%s: no kubernetes runtime configuration available", selector.Description())
	}

	var err error
	hostEnv.Kubeconfig, err = dirs.ExpandHome(hostEnv.Kubeconfig)
	if err != nil {
		return nil, fnerrors.InternalError("failed to expand %q", hostEnv.Kubeconfig)
	}

	return &HostConfig{DevHost: devHost, Selector: selector, HostEnv: hostEnv}, nil
}
