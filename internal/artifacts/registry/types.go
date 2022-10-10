// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	mapping = map[string]func(context.Context, planning.Configuration) (Manager, error){}

	ErrNoRegistry = errors.New("no registry configured")

	registryConfigType         = planning.DefineConfigType[*registry.Registry]()
	registryProviderConfigType = planning.DefineConfigType[*registry.Provider]()
)

// XXX use external plugin system.
func Register(name string, make func(context.Context, planning.Configuration) (Manager, error)) {
	mapping[strings.ToLower(name)] = make
}

type Manager interface {
	// Returns true if calls to the registry should be made over HTTP (instead of HTTPS).
	IsInsecure() bool

	AllocateName(repository string) compute.Computable[oci.AllocatedName]
	AuthRepository(oci.ImageID) (oci.AllocatedName, error)
}

func GetRegistry(ctx context.Context, env planning.Context) (Manager, error) {
	return GetRegistryFromConfig(ctx, env.Environment().Name, env.Configuration())
}

func GetRegistryFromConfig(ctx context.Context, env string, cfg planning.Configuration) (Manager, error) {
	r, ok := registryConfigType.CheckGet(cfg)

	if ok && r.Url != "" {
		if trimmed := strings.TrimPrefix(r.Url, "http://"); trimmed != r.Url {
			r.Url = trimmed
			r.Insecure = true
		}
		return MakeStaticRegistry(r), nil
	}

	p, ok := registryProviderConfigType.CheckGet(cfg)
	if ok && p.Provider != "" {
		return GetRegistryByName(ctx, cfg, p.Provider)
	}

	if env == "" {
		return nil, ErrNoRegistry
	}

	return nil, fnerrors.UsageError(
		fmt.Sprintf("Run `ns prepare local --env=%s` to set it up.", env),
		"No registry configured in the environment %q.", env)
}

func GetRegistryByName(ctx context.Context, conf planning.Configuration, name string) (Manager, error) {
	if m, ok := mapping[name]; ok {
		return m(ctx, conf)
	}

	return nil, fnerrors.UserError(nil, "%q is not a known registry provider", name)
}

func StaticName(registry *registry.Registry, imageID oci.ImageID, keychain oci.Keychain) compute.Computable[oci.AllocatedName] {
	return compute.Map(tasks.Action("registry.allocate-tag").Arg("ref", imageID.ImageRef()), compute.Inputs().
		JSON("imageID", imageID).
		Indigestible("registry", registry).Indigestible("keychain", keychain),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, r compute.Resolved) (oci.AllocatedName, error) {
			return oci.AllocatedName{
				InsecureRegistry: registry.GetInsecure(),
				ImageID:          imageID,
				Keychain:         keychain,
			}, nil
		})
}

func AllocateName(ctx context.Context, env planning.Context, pkg schema.PackageName) (compute.Computable[oci.AllocatedName], error) {
	registry, err := GetRegistry(ctx, env)
	if err != nil {
		return nil, err
	}

	return registry.AllocateName(pkg.String()), nil
}

func RawAllocateName(ctx context.Context, ck planning.Configuration, repo string) (compute.Computable[oci.AllocatedName], error) {
	registry, err := GetRegistryFromConfig(ctx, "", ck)
	if err != nil {
		return nil, err
	}

	return registry.AllocateName(repo), nil
}

func Precomputed(tag oci.AllocatedName) compute.Computable[oci.AllocatedName] {
	return precomputedTag{tag: tag}
}

type precomputedTag struct {
	tag oci.AllocatedName
	compute.PrecomputeScoped[oci.AllocatedName]
}

var _ compute.Digestible = precomputedTag{}

func (r precomputedTag) Inputs() *compute.In {
	return compute.Inputs().JSON("tag", r.tag)
}

func (r precomputedTag) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (r precomputedTag) Action() *tasks.ActionEvent {
	return tasks.Action("registry.tag").Arg("ref", r.tag.ImageRef())
}

func (r precomputedTag) Compute(ctx context.Context, _ compute.Resolved) (oci.AllocatedName, error) {
	return r.tag, nil
}

func (r precomputedTag) ComputeDigest(ctx context.Context) (schema.Digest, error) {
	return r.tag.ComputeDigest(ctx)
}
