// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package registry

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

var (
	mapping = map[string]func(context.Context, cfg.Configuration) (Manager, error){}

	ErrNoRegistry = errors.New("no registry configured")

	registryConfigType         = cfg.DefineConfigType[*registry.Registry]()
	registryProviderConfigType = cfg.DefineConfigType[*registry.Provider]()
)

// XXX use external plugin system.
func Register(name string, make func(context.Context, cfg.Configuration) (Manager, error)) {
	mapping[strings.ToLower(name)] = make
}

type Manager interface {
	Access() oci.RegistryAccess

	AllocateName(repository string) compute.Computable[oci.RepositoryWithParent]
}

func GetRegistry(ctx context.Context, env cfg.Context) (Manager, error) {
	return GetRegistryFromConfig(ctx, env.Environment().Name, env.Configuration())
}

func GetRegistryFromConfig(ctx context.Context, env string, cfg cfg.Configuration) (Manager, error) {
	p, ok := registryProviderConfigType.CheckGet(cfg)
	if ok && p.Provider != "" {
		return getRegistryByName(ctx, cfg, p.Provider)
	}

	r, ok := registryConfigType.CheckGet(cfg)
	if ok && r.Url != "" {
		if trimmed := strings.TrimPrefix(r.Url, "http://"); trimmed != r.Url {
			r.Url = trimmed
			r.Insecure = true
		}

		if r.Transport != nil && r.Transport.Ssh != nil {
			if r.Transport.Ssh.RemoteAddr == "" {
				// We pass a dummy scheme just to facilitate parsing.
				parsed, err := url.Parse("//" + r.Url)
				if err != nil {
					return nil, fnerrors.New("transport.ssh: failed to compute remote address while parsing url: %w", err)
				}
				r.Transport.Ssh.RemoteAddr = parsed.Host
			}
		}

		return MakeStaticRegistry(r), nil
	}

	if env == "" {
		return nil, ErrNoRegistry
	}

	return nil, fnerrors.UsageError(
		fmt.Sprintf("Run `ns prepare local --env=%s` to set it up.", env),
		"No registry configured in the environment %q.", env)
}

func getRegistryByName(ctx context.Context, conf cfg.Configuration, name string) (Manager, error) {
	if m, ok := mapping[name]; ok {
		return m(ctx, conf)
	}

	return nil, fnerrors.New("%q is not a known registry provider", name)
}

func StaticRepository(parent Manager, repository string, access oci.RegistryAccess) compute.Computable[oci.RepositoryWithParent] {
	return compute.Map(tasks.Action("registry.static-repository").Arg("repository", repository), compute.Inputs().
		Bool("insecure", access.InsecureRegistry).
		Proto("transport", access.Transport).
		JSON("repository", repository).
		Indigestible("parent", parent).
		Indigestible("keychain", access.Keychain),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, r compute.Resolved) (oci.RepositoryWithParent, error) {
			return oci.RepositoryWithParent{
				Parent: parent,
				RepositoryWithAccess: oci.RepositoryWithAccess{
					RegistryAccess: access,
					Repository:     repository,
				},
			}, nil
		})
}

func Precomputed(tag oci.RepositoryWithParent) compute.Computable[oci.RepositoryWithParent] {
	return precomputedTag{tag: tag}
}

type precomputedTag struct {
	tag oci.RepositoryWithParent
	compute.PrecomputeScoped[oci.RepositoryWithParent]
}

var _ compute.Digestible = precomputedTag{}

func (r precomputedTag) Inputs() *compute.In {
	return compute.Inputs().JSON("tag", r.tag)
}

func (r precomputedTag) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (r precomputedTag) Action() *tasks.ActionEvent {
	return tasks.Action("registry.precomputed-repository").Arg("repository", r.tag.Repository)
}

func (r precomputedTag) Compute(ctx context.Context, _ compute.Resolved) (oci.RepositoryWithParent, error) {
	return r.tag, nil
}

func (r precomputedTag) ComputeDigest(ctx context.Context) (schema.Digest, error) {
	return r.tag.ComputeDigest(ctx)
}
