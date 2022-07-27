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
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Provider struct {
	Config        clientcmdapi.Config
	TokenProvider TokenProviderFunc
}

type TokenProviderFunc func(context.Context) (string, error)

type ProviderFunc func(context.Context, *fnschema.Environment, *devhost.ConfigKey) (Provider, error)

var (
	clientCache struct {
		mu    sync.Mutex
		cache map[string]*k8s.Clientset
	}
	providers = map[string]ProviderFunc{}
)

func init() {
	clientCache.cache = map[string]*k8s.Clientset{}
}

func RegisterProvider(name string, p ProviderFunc) {
	providers[name] = p
}

func NewRestConfigFromHostEnv(ctx context.Context, host *HostConfig) (*rest.Config, error) {
	return NewClientConfig(ctx, host).ClientConfig()
}

func NewClientConfig(ctx context.Context, host *HostConfig) clientcmd.ClientConfig {
	return &configWithToken{ctx: ctx, host: host}
}

type configWithToken struct {
	ctx  context.Context
	host *HostConfig

	mu            sync.Mutex
	computed      clientcmd.ClientConfig
	tokenProvider TokenProviderFunc
}

func (cfg *configWithToken) computeConfig() (clientcmd.ClientConfig, TokenProviderFunc, error) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	if cfg.computed != nil {
		return cfg.computed, cfg.tokenProvider, nil
	}

	c := cfg.host.HostEnv

	if c.Provider != "" {
		p := providers[c.Provider]
		if p == nil {
			return nil, nil, fnerrors.BadInputError("%s: no such kubernetes configuration provider", c.Provider)
		}

		cached := &cachedProviderConfig{
			providerName: c.Provider,
			env:          cfg.host.Environment,
			configKey:    &devhost.ConfigKey{DevHost: cfg.host.DevHost, Selector: cfg.host.Selector},
			provider:     p,
		}

		x, err := compute.GetValue[Provider](cfg.ctx, cached)
		if err != nil {
			return nil, nil, err
		}

		cfg.computed = clientcmd.NewDefaultClientConfig(x.Config, nil)
		cfg.tokenProvider = x.TokenProvider
		return cfg.computed, cfg.tokenProvider, nil
	}

	if c.GetKubeconfig() == "" {
		return nil, nil, fnerrors.New("hostEnv.Kubeconfig is required")
	}

	cfg.computed = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: c.GetKubeconfig()},
		&clientcmd.ConfigOverrides{CurrentContext: c.GetContext()})

	return cfg.computed, nil, nil
}

func (cfg *configWithToken) RawConfig() (clientcmdapi.Config, error) {
	x, _, err := cfg.computeConfig()
	if err != nil {
		return clientcmdapi.Config{}, err
	}

	return x.RawConfig()
}

func (cfg *configWithToken) ClientConfig() (*rest.Config, error) {
	if cfg.host.HostEnv.GetIncluster() {
		return rest.InClusterConfig()
	}

	x, tokenProvider, err := cfg.computeConfig()
	if err != nil {
		return nil, err
	}

	computed, err := x.ClientConfig()
	if err != nil {
		return nil, err
	}

	if tokenProvider != nil {
		token, err := tokenProvider(cfg.ctx)
		if err != nil {
			return nil, err
		}

		computed.BearerToken = token
	}

	return computed, nil
}

func (cfg *configWithToken) Namespace() (string, bool, error) {
	x, _, err := cfg.computeConfig()
	if err != nil {
		return "", false, err
	}
	return x.Namespace()
}

func (cfg *configWithToken) ConfigAccess() clientcmd.ConfigAccess {
	panic("ConfigAccess is not implemented")
}

func NewClient(ctx context.Context, host *HostConfig) (*k8s.Clientset, error) {
	if host.Selector == nil {
		config, err := NewRestConfigFromHostEnv(ctx, host)
		if err != nil {
			return nil, err
		}

		return k8s.NewForConfig(config)
	}

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

	cfg, err := ComputeHostConfig(env.Proto(), env.DevHost(), devhost.ByEnvironment(env.Proto()))
	if err != nil {
		return nil, err
	}

	return NewRestConfigFromHostEnv(ctx, cfg)
}

func ComputeHostConfig(env *fnschema.Environment, devHost *fnschema.DevHost, selector devhost.Selector) (*HostConfig, error) {
	cfg := devhost.Select(devHost, selector)

	hostEnv := &HostEnv{}
	if !cfg.Get(hostEnv) {
		if err := devhost.CheckEmptyErr(devHost); err != nil {
			return nil, err
		}

		return nil, fnerrors.UsageError("Try running one `ns prepare local` or `ns prepare eks`", "%s: no kubernetes configuration available", selector.Description())
	}

	var err error
	hostEnv.Kubeconfig, err = dirs.ExpandHome(hostEnv.Kubeconfig)
	if err != nil {
		return nil, fnerrors.InternalError("failed to expand %q", hostEnv.Kubeconfig)
	}

	return &HostConfig{Environment: env, DevHost: devHost, Selector: selector, HostEnv: hostEnv}, nil
}

// Only compute configurations once per `ns` invocation.
type cachedProviderConfig struct {
	providerName string
	env          *fnschema.Environment
	configKey    *devhost.ConfigKey

	provider ProviderFunc

	compute.DoScoped[Provider]
}

var _ compute.Computable[Provider] = &cachedProviderConfig{}

func (t *cachedProviderConfig) Action() *tasks.ActionEvent {
	return tasks.Action("kubernetes.compute-config").Arg("provider", t.providerName)
}
func (t *cachedProviderConfig) Inputs() *compute.In {
	return compute.Inputs().Str("provider", t.providerName).Proto("devhost", t.configKey.DevHost).Proto("env", t.env)
}
func (t *cachedProviderConfig) Output() compute.Output { return compute.Output{NotCacheable: true} }
func (t *cachedProviderConfig) Compute(ctx context.Context, _ compute.Resolved) (Provider, error) {
	return t.provider(ctx, t.env, t.configKey)
}
