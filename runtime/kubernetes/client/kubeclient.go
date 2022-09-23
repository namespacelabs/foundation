// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"context"
	"fmt"

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

type Prepared struct {
	Clientset     *k8s.Clientset
	RESTConfig    *rest.Config
	ClientConfig  clientcmd.ClientConfig
	HostEnv       *HostEnv
	Configuration ClusterConfiguration
}

func RegisterConfigurationProvider(name string, p ProviderFunc) {
	providers[name] = p
}

type configResult struct {
	ClientConfig clientcmd.ClientConfig
	ClusterConfiguration
}

func computeConfig(ctx context.Context, c *HostEnv, config planning.Configuration) (*configResult, error) {
	if c.Incluster {
		return nil, nil
	}

	if c.Provider != "" {
		provider := providers[c.Provider]
		if provider == nil {
			return nil, fnerrors.BadInputError("%s: no such kubernetes configuration provider", c.Provider)
		}

		result, err := provider(ctx, config)
		if err != nil {
			return nil, err
		}

		return &configResult{
			ClientConfig:         clientcmd.NewDefaultClientConfig(result.Config, nil),
			ClusterConfiguration: result,
		}, nil
	}

	if c.StaticConfig != nil {
		return &configResult{
			ClientConfig: clientcmd.NewDefaultClientConfig(*MakeApiConfig(c.StaticConfig), nil),
		}, nil
	}

	if c.GetKubeconfig() == "" {
		return nil, fnerrors.New("hostEnv.Kubeconfig is required")
	}

	kubeconfig, err := dirs.ExpandHome(c.GetKubeconfig())
	if err != nil {
		return nil, err
	}

	return &configResult{
		ClientConfig: clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			&clientcmd.ConfigOverrides{CurrentContext: c.GetContext()}),
	}, nil
}

func obtainRESTConfig(ctx context.Context, hostEnv *HostEnv, computed *configResult) (*rest.Config, error) {
	if hostEnv.GetIncluster() {
		config, err := rest.InClusterConfig()
		return config, err
	}

	restcfg, err := computed.ClientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	if computed.TokenProvider != nil {
		token, err := computed.TokenProvider(ctx)
		if err != nil {
			return nil, err
		}

		restcfg.BearerToken = token
	}

	return restcfg, nil
}

func NewClient(ctx context.Context, cfg planning.Configuration) (*Prepared, error) {
	fmt.Fprintf(console.Debug(ctx), "kubernetes.NewClient\n")

	hostEnv, err := CheckGetHostEnv(cfg)
	if err != nil {
		return nil, err
	}

	computed, err := computeConfig(ctx, hostEnv, cfg)
	if err != nil {
		return nil, err
	}

	restcfg, err := obtainRESTConfig(ctx, hostEnv, computed)
	if err != nil {
		return nil, err
	}

	clientset, err := k8s.NewForConfig(restcfg)
	if err != nil {
		return nil, err
	}

	c := &Prepared{
		Clientset:  clientset,
		RESTConfig: restcfg,
		HostEnv:    hostEnv,
	}

	if computed != nil {
		c.Configuration = computed.ClusterConfiguration
		c.ClientConfig = computed.ClientConfig
	}

	return c, nil
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
