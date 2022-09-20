// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"context"
	"fmt"
	sync "sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/dirs"
)

type ClusterConfiguration struct {
	Config           clientcmdapi.Config
	TokenProvider    TokenProviderFunc
	Ephemeral        bool // Set to true if thie target cluster is ephemeral.
	ProviderSpecific any  // Up to an implementation to attach state if needed.
	Labels           []*fnschema.Label
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
	internal   *configResult
}

func (cc ComputedClient) ClusterConfiguration() ClusterConfiguration {
	if cc.internal == nil {
		return ClusterConfiguration{}
	}

	return cc.internal.ClusterConfiguration
}

func (cc ComputedClient) ClientConfig() clientcmd.ClientConfig {
	if cc.internal == nil {
		return nil
	}

	return cc.internal.ClientConfig
}

func (cc ComputedClient) Ephemeral() bool {
	if cc.internal == nil {
		return false
	}

	return cc.internal.Ephemeral
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
	ClientConfig clientcmd.ClientConfig
	ClusterConfiguration
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

		result, err := provider(cfg.ctx, cfg.host.Config)
		if err != nil {
			return nil, err
		}

		cfg.computed = &configResult{
			ClientConfig:         clientcmd.NewDefaultClientConfig(result.Config, nil),
			ClusterConfiguration: result,
		}

		return cfg.computed, nil
	}

	if c.StaticConfig != nil {
		cfg.computed = &configResult{
			ClientConfig: clientcmd.NewDefaultClientConfig(*MakeApiConfig(c.StaticConfig), nil),
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

func (cfg *computedConfig) ClientConfigAndInternal() (*configResult, *rest.Config, error) {
	if cfg.host.HostEnv.GetIncluster() {
		config, err := rest.InClusterConfig()
		return nil, config, err
	}

	x, err := cfg.computeConfig()
	if err != nil {
		return nil, nil, err
	}

	computed, err := x.ClientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	if x.TokenProvider != nil {
		token, err := x.TokenProvider(cfg.ctx)
		if err != nil {
			return nil, nil, err
		}

		computed.BearerToken = token
	}

	return x, computed, nil
}

func (cfg *computedConfig) ClientConfig() (*rest.Config, error) {
	_, config, err := cfg.ClientConfigAndInternal()
	return config, err
}

func (cfg *computedConfig) Namespace() (string, bool, error) {
	x, err := cfg.computeConfig()
	if err != nil {
		return "", false, err
	}
	return x.ClientConfig.Namespace()
}

func NewClient(ctx context.Context, host *HostConfig) (*ComputedClient, error) {
	fmt.Fprintf(console.Debug(ctx), "kubernetes.NewClient\n")

	computed, config, err := NewClientConfig(ctx, host).ClientConfigAndInternal()
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
		internal:   computed,
	}, nil
}

func MakeGroupVersionBasedClientAndConfig(ctx context.Context, original *rest.Config, gv schema.GroupVersion) (*rest.Config, rest.Interface, error) {
	config := copyAndSetDefaults(*original, gv)
	client, err := rest.RESTClientFor(config)
	return config, client, err
}

func MakeGroupVersionBasedClient(ctx context.Context, original *rest.Config, gv schema.GroupVersion) (rest.Interface, error) {
	_, cli, err := MakeGroupVersionBasedClientAndConfig(ctx, original, gv)
	return cli, err
}

func copyAndSetDefaults(config rest.Config, gv schema.GroupVersion) *rest.Config {
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
