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
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Provider struct {
	Config           clientcmdapi.Config
	TokenProvider    TokenProviderFunc
	ProviderSpecific any // Up to an implementation to attach state if needed.
}

type DeferredProvider struct{}

type TokenProviderFunc func(context.Context) (string, error)

type DeferredProviderFunc func(context.Context, *HostConfig) (runtime.DeferredRuntime, error)
type ProviderFunc func(context.Context, planning.Configuration) (Provider, error)

var (
	clientCache struct {
		mu    sync.Mutex
		cache map[string]*ComputedClient
	}
	providers         = map[string]ProviderFunc{}
	deferredProviders = map[string]DeferredProviderFunc{}
)

type ComputedClient struct {
	Clientset *k8s.Clientset
	parent    *computedConfig
}

func (cc ComputedClient) Provider() (Provider, error) {
	if cc.parent == nil {
		return Provider{}, nil
	}

	x, err := cc.parent.computeConfig()
	if err != nil {
		return Provider{}, err
	}

	return x.Provider, nil
}

func init() {
	clientCache.cache = map[string]*ComputedClient{}
}

func RegisterProvider(name string, p ProviderFunc) {
	providers[name] = p
}

func RegisterDeferredProvider(name string, p DeferredProviderFunc) {
	deferredProviders[name] = p
}

func NewRestConfigFromHostEnv(ctx context.Context, host *HostConfig) (*rest.Config, error) {
	return NewClientConfig(ctx, host).ClientConfig()
}

func NewClientConfig(ctx context.Context, host *HostConfig) *computedConfig {
	return &computedConfig{ctx: ctx, host: host}
}

type computedConfig struct {
	ctx  context.Context
	host *HostConfig

	mu       sync.Mutex
	computed *configResult
}

type configResult struct {
	ClientConfig  clientcmd.ClientConfig
	TokenProvider TokenProviderFunc
	Provider      Provider
}

func (cfg *computedConfig) computeConfig() (*configResult, error) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	if cfg.computed != nil {
		return cfg.computed, nil
	}

	c := cfg.host.HostEnv

	if c.Provider != "" {
		p := providers[c.Provider]
		if p == nil {
			return nil, fnerrors.BadInputError("%s: no such kubernetes configuration provider", c.Provider)
		}

		cached := &cachedProviderConfig{
			providerName: c.Provider,
			config:       cfg.host.Config,
			provider:     p,
		}

		x, err := compute.GetValue[Provider](cfg.ctx, cached)
		if err != nil {
			return nil, err
		}

		cfg.computed = &configResult{
			ClientConfig:  clientcmd.NewDefaultClientConfig(x.Config, nil),
			TokenProvider: x.TokenProvider,
			Provider:      x,
		}

		return cfg.computed, nil
	}

	if c.GetKubeconfig() == "" {
		return nil, fnerrors.New("hostEnv.Kubeconfig is required")
	}

	cfg.computed = &configResult{
		ClientConfig: clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: c.GetKubeconfig()},
			&clientcmd.ConfigOverrides{CurrentContext: c.GetContext()}),
	}

	return cfg.computed, nil
}

func (cfg *computedConfig) RawConfig() (clientcmdapi.Config, error) {
	x, err := cfg.computeConfig()
	if err != nil {
		return clientcmdapi.Config{}, err
	}

	return x.ClientConfig.RawConfig()
}

func (cfg *computedConfig) ClientConfig() (*rest.Config, error) {
	if cfg.host.HostEnv.GetIncluster() {
		return rest.InClusterConfig()
	}

	x, err := cfg.computeConfig()
	if err != nil {
		return nil, err
	}

	computed, err := x.ClientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	if x.TokenProvider != nil {
		token, err := x.TokenProvider(cfg.ctx)
		if err != nil {
			return nil, err
		}

		computed.BearerToken = token
	}

	return computed, nil
}

func (cfg *computedConfig) Namespace() (string, bool, error) {
	x, err := cfg.computeConfig()
	if err != nil {
		return "", false, err
	}
	return x.ClientConfig.Namespace()
}

func (cfg *computedConfig) ConfigAccess() clientcmd.ConfigAccess {
	panic("ConfigAccess is not implemented")
}

func NewClient(ctx context.Context, host *HostConfig) (*ComputedClient, error) {
	keyBytes, err := json.Marshal(struct {
		C *HostEnv
		S string
	}{host.HostEnv, host.Config.HashKey()})
	if err != nil {
		return nil, fnerrors.InternalError("failed to serialize config/env key: %w", err)
	}

	key := string(keyBytes)

	clientCache.mu.Lock()
	defer clientCache.mu.Unlock()

	if _, ok := clientCache.cache[key]; !ok {
		parent := NewClientConfig(ctx, host)

		config, err := parent.ClientConfig()
		if err != nil {
			return nil, err
		}

		clientset, err := k8s.NewForConfig(config)
		if err != nil {
			return nil, err
		}

		clientCache.cache[key] = &ComputedClient{
			Clientset: clientset,
			parent:    parent,
		}
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

type ResourceClassLike interface {
	GetResource() string
	GetResourceClass() *kubedef.ResourceClass
}

func MakeGroupVersionBasedClient(ctx context.Context, gv schema.GroupVersion, cfg *rest.Config) (rest.Interface, error) {
	return rest.RESTClientFor(CopyAndSetDefaults(*cfg, gv))
}

func MakeResourceSpecificClient(ctx context.Context, resource ResourceClassLike, cfg *rest.Config) (rest.Interface, error) {
	if klass := resource.GetResourceClass(); klass != nil {
		return MakeGroupVersionBasedClient(ctx, klass.GroupVersion(), cfg)
	}

	gv, ok := groups[resource.GetResource()]
	if !ok {
		return nil, fnerrors.InternalError("%s: don't know how to construct a client for this resource", resource.GetResource())
	}

	return rest.RESTClientFor(CopyAndSetDefaults(*cfg, gv))
}

func CopyAndSetDefaults(config rest.Config, gv schema.GroupVersion) *rest.Config {
	config.GroupVersion = &gv
	if gv.Group == "" {
		config.APIPath = "/api"
	} else {
		config.APIPath = "/apis"
	}
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return &config
}

func ResolveConfig(ctx context.Context, env planning.Context) (*rest.Config, error) {
	if x, ok := env.(interface {
		KubeconfigProvider() (*HostConfig, error)
	}); ok {
		cfg, err := x.KubeconfigProvider()
		if err != nil {
			return nil, err
		}

		return NewRestConfigFromHostEnv(ctx, cfg)
	}

	cfg, err := ComputeHostConfig(env.Configuration())
	if err != nil {
		return nil, err
	}

	return NewRestConfigFromHostEnv(ctx, cfg)
}

func ComputeHostConfig(cfg planning.Configuration) (*HostConfig, error) {
	hostEnv := &HostEnv{}
	if !cfg.Get(hostEnv) {
		return nil, fnerrors.UsageError("Try running one `ns prepare local` or `ns prepare eks`", "%s: no kubernetes configuration available", cfg.EnvKey())
	}

	var err error
	hostEnv.Kubeconfig, err = dirs.ExpandHome(hostEnv.Kubeconfig)
	if err != nil {
		return nil, fnerrors.InternalError("failed to expand %q", hostEnv.Kubeconfig)
	}

	return &HostConfig{Config: cfg, HostEnv: hostEnv}, nil
}

func MakeDeferredRuntime(ctx context.Context, cfg *HostConfig) (runtime.DeferredRuntime, error) {
	if cfg.HostEnv.Provider != "" {
		if p := deferredProviders[cfg.HostEnv.Provider]; p != nil {
			return p(ctx, cfg)
		}
	}

	return nil, nil
}

// Only compute configurations once per `ns` invocation.
type cachedProviderConfig struct {
	providerName string
	config       planning.Configuration

	provider ProviderFunc

	compute.DoScoped[Provider]
}

var _ compute.Computable[Provider] = &cachedProviderConfig{}

func (t *cachedProviderConfig) Action() *tasks.ActionEvent {
	return tasks.Action("kubernetes.compute-config").Arg("provider", t.providerName)
}
func (t *cachedProviderConfig) Inputs() *compute.In {
	return compute.Inputs().Str("provider", t.providerName).
		Str("configHash", t.config.HashKey()). // We depend on the configuration cache keys being stable.
		Str("config", t.config.EnvKey())
}
func (t *cachedProviderConfig) Output() compute.Output { return compute.Output{NotCacheable: true} }
func (t *cachedProviderConfig) Compute(ctx context.Context, _ compute.Resolved) (Provider, error) {
	return t.provider(ctx, t.config)
}
