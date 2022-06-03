// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"context"

	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	tokenProviders = map[string]func(context.Context, *ConfigKey) (string, error){}
)

type ConfigKey struct {
	DevHost  *schema.DevHost
	Selector devhost.Selector
}

func RegisterBearerTokenProvider(name string, provider func(context.Context, *ConfigKey) (string, error)) {
	tokenProviders[name] = provider
}

func computeBearerToken(ctx context.Context, cfg *HostConfig, out *rest.Config) error {
	if cfg.HostEnv.BearerTokenProvider == "" {
		return nil
	}

	provider, ok := tokenProviders[cfg.HostEnv.BearerTokenProvider]
	if !ok {
		return fnerrors.BadInputError("%s: not a known kubernetes bearer token provider", cfg.HostEnv.BearerTokenProvider)
	}

	token, err := compute.GetValue[string](ctx, &cachedToken{
		providerName: cfg.HostEnv.BearerTokenProvider,
		configKey:    &ConfigKey{DevHost: cfg.DevHost, Selector: cfg.Selector},
		provider:     provider,
	})
	if err != nil {
		return fnerrors.New("%s: failed: %w", cfg.HostEnv.BearerTokenProvider, err)
	}

	out.BearerToken = token
	return nil
}

// Only compute bearer token once per `fn` invocation.
type cachedToken struct {
	providerName string
	configKey    *ConfigKey

	provider func(context.Context, *ConfigKey) (string, error)

	compute.DoScoped[string]
}

var _ compute.Computable[string] = &cachedToken{}

func (t *cachedToken) Action() *tasks.ActionEvent {
	return tasks.Action("kubernetes.compute-bearer-token").Arg("provider", t.providerName)
}
func (t *cachedToken) Inputs() *compute.In {
	return compute.Inputs().Str("provider", t.providerName).Proto("devhost", t.configKey.DevHost)
}
func (t *cachedToken) Output() compute.Output { return compute.Output{NotCacheable: true} }
func (t *cachedToken) Compute(ctx context.Context, _ compute.Resolved) (string, error) {
	return t.provider(ctx, t.configKey)
}
