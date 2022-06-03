// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"context"
	"fmt"

	k8s "k8s.io/client-go/kubernetes"
	tadmissionregistrationv1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	tappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	tbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	tcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	tnetworkv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	trbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/providers/aws/auth"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
)

type KubeconfigProvider interface {
	GetKubeconfig() string
	GetContext() string
	GetIncluster() bool
}

func NewRestConfigFromHostEnv(cfg KubeconfigProvider) (*restclient.Config, error) {
	if cfg.GetIncluster() {
		return restclient.InClusterConfig()
	}

	if cfg.GetKubeconfig() == "" {
		return nil, fnerrors.New("hostEnv.Kubeconfig is required")
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.GetKubeconfig()},
		&clientcmd.ConfigOverrides{CurrentContext: cfg.GetContext()}).ClientConfig()
}

func NewClient(ctx context.Context, cfg KubeconfigProvider, err error) (*k8s.Clientset, error) {
	if err != nil {
		return nil, err
	}

	return NewClientFromHostEnv(ctx, cfg)
}

func NewClientFromHostEnv(ctx context.Context, cfg KubeconfigProvider) (*k8s.Clientset, error) {
	config, err := NewRestConfigFromHostEnv(cfg)
	if err != nil {
		return nil, err
	}

	// If the auth provider is AWS, check for credentials on our side first.
	// This will allow us to produce a better error message, if the credentials
	// are out of date. In the future, our AWS integration may also issue tokens,
	// which amortize this cost.
	//
	// This uses an heuristic: if the configuration is setup to use an exec-based
	// authenticator that is called "aws-iam-authenticator", then we're likely
	// hitting EKS. But if the user sets up some other authenticator, that's fine,
	// it still work, but any information
	if config.ExecProvider != nil && config.ExecProvider.Command == "aws-iam-authenticator" {
		var profile string
		for _, kv := range config.ExecProvider.Env {
			if kv.Name == "AWS_PROFILE" {
				profile = kv.Value
			}
		}

		r, err := auth.ResolveWithProfile(ctx, profile)
		if err != nil {
			return nil, fnerrors.Wrap(nil, err)
		}

		// If we're not able to resolve our own account, then auth is likely needed. The error
		// will contain user instructions.
		if _, err := compute.GetValue(ctx, r); err != nil {
			return nil, fnerrors.Wrap(nil, err)
		}
	}

	clientset, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
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

func ResolveConfig(env ops.Environment) (*restclient.Config, error) {
	if x, ok := env.(interface {
		KubeconfigProvider() (KubeconfigProvider, error)
	}); ok {
		provider, err := x.KubeconfigProvider()
		if err != nil {
			return nil, err
		}
		return NewRestConfigFromHostEnv(provider)
	}

	cfg, err := ComputeHostEnv(env.DevHost(), env.Proto())
	if err != nil {
		return nil, err
	}

	return NewRestConfigFromHostEnv(cfg)
}

func ComputeHostEnv(devHost *schema.DevHost, env *schema.Environment) (*HostEnv, error) {
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

	return hostEnv, nil
}

func ConfigFromEnv(ctx context.Context, env ops.Environment) (context.Context, KubeconfigProvider, error) {
	if x, ok := env.(interface {
		KubeconfigProvider() (KubeconfigProvider, error)
	}); ok {
		p, err := x.KubeconfigProvider()
		return ctx, p, err
	}
	return ConfigFromDevHost(ctx, env.DevHost(), env.Proto())
}

func ConfigFromDevHost(ctx context.Context, devhost *schema.DevHost, env *schema.Environment) (context.Context, KubeconfigProvider, error) {
	cfg, err := ComputeHostEnv(devhost, env)
	if err != nil {
		return ctx, nil, err
	}

	fmt.Fprintf(console.Debug(ctx), "kubernetes: using configuration: %+v\n", cfg)

	return ctx, cfg, nil
}
