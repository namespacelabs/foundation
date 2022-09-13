// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"context"
	"fmt"
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
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/dirs"
)

type ClusterConfiguration struct {
	Config           clientcmdapi.Config
	TokenProvider    TokenProviderFunc
	ProviderSpecific any // Up to an implementation to attach state if needed.
}

type DeferredProvider struct{}

type TokenProviderFunc func(context.Context) (string, error)

type ProviderFunc func(context.Context, planning.Configuration) (ClusterConfiguration, error)

var (
	providers = map[string]ProviderFunc{}
)

type ComputedClient struct {
	Clientset  *k8s.Clientset
	RESTConfig *rest.Config
	parent     *computedConfig
}

func (cc ComputedClient) Provider() (ClusterConfiguration, error) {
	if cc.parent == nil {
		return ClusterConfiguration{}, nil
	}

	x, err := cc.parent.computeConfig()
	if err != nil {
		return ClusterConfiguration{}, err
	}

	return x.Provider, nil
}

func RegisterConfigurationProvider(name string, p ProviderFunc) {
	providers[name] = p
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
	Provider      ClusterConfiguration
}

func (cfg *computedConfig) computeConfig() (*configResult, error) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	if cfg.computed != nil {
		return cfg.computed, nil
	}

	c := cfg.host.HostEnv

	if c.Provider != "" {
		provider := providers[c.Provider]
		if provider == nil {
			return nil, fnerrors.BadInputError("%s: no such kubernetes configuration provider", c.Provider)
		}

		x, err := provider(cfg.ctx, cfg.host.Config)
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
	fmt.Fprintf(console.Debug(ctx), "kubernetes.NewClient\n")

	parent := NewClientConfig(ctx, host)

	config, err := parent.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &ComputedClient{
		Clientset:  clientset,
		RESTConfig: config,
		parent:     parent,
	}, nil
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

func CheckGetHostEnv(cfg planning.Configuration) (*HostEnv, error) {
	hostEnv := &HostEnv{}
	if !cfg.Get(hostEnv) {
		return nil, fnerrors.UsageError("Try running one `ns prepare local` or `ns prepare eks`", "%s: no kubernetes configuration available", cfg.EnvKey())
	}
	return hostEnv, nil
}

func ComputeHostConfig(cfg planning.Configuration) (*HostConfig, error) {
	hostEnv, err := CheckGetHostEnv(cfg)
	if err != nil {
		return nil, err
	}

	hostEnv.Kubeconfig, err = dirs.ExpandHome(hostEnv.Kubeconfig)
	if err != nil {
		return nil, fnerrors.InternalError("failed to expand %q", hostEnv.Kubeconfig)
	}

	return &HostConfig{Config: cfg, HostEnv: hostEnv}, nil
}
