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
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	mapping = map[string]func(context.Context, *devhost.ConfigKey) (Manager, error){}

	ErrNoRegistry = errors.New("no registry configured")
)

// XXX use external plugin system.
func Register(name string, make func(context.Context, *devhost.ConfigKey) (Manager, error)) {
	mapping[strings.ToLower(name)] = make
}

type Manager interface {
	// Returns true if calls to the registry should be made over HTTP (instead of HTTPS).
	IsInsecure() bool

	AllocateName(repository string) compute.Computable[oci.AllocatedName]
	AuthRepository(oci.ImageID) (oci.AllocatedName, error)
}

func GetRegistry(ctx context.Context, env ops.Environment) (Manager, error) {
	return GetRegistryFromConfig(ctx, &devhost.ConfigKey{
		DevHost:  env.DevHost(),
		Selector: devhost.ByEnvironment(env.Proto()),
	})
}

func GetRegistryFromConfig(ctx context.Context, conf *devhost.ConfigKey) (Manager, error) {
	cfg := conf.Selector.Select(conf.DevHost)

	r := &registry.Registry{}
	if cfg.Get(r) && r.Url != "" {
		if trimmed := strings.TrimPrefix(r.Url, "http://"); trimmed != r.Url {
			r.Url = trimmed
			r.Insecure = true
		}
		return staticRegistry{r}, nil
	}

	p := &registry.Provider{}
	if cfg.Get(p) && p.Provider != "" {
		return GetRegistryByName(ctx, conf, p.Provider)
	}

	return nil, nil
}

func GetRegistryByName(ctx context.Context, conf *devhost.ConfigKey, name string) (Manager, error) {
	if m, ok := mapping[name]; ok {
		return m(ctx, conf)
	}

	return nil, fnerrors.UserError(nil, "%q is not a known registry provider", name)
}

func StaticName(registry *registry.Registry, imageID oci.ImageID) compute.Computable[oci.AllocatedName] {
	return compute.Map(tasks.Action("registry.allocate-tag"), compute.Inputs(),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, r compute.Resolved) (oci.AllocatedName, error) {
			return oci.AllocatedName{
				InsecureRegistry: registry.GetInsecure(),
				ImageID:          imageID,
			}, nil
		})
}

func AllocateName(ctx context.Context, env ops.Environment, pkg schema.PackageName) (compute.Computable[oci.AllocatedName], error) {
	allocated, err := RawAllocateName(ctx, &devhost.ConfigKey{
		DevHost:  env.DevHost(),
		Selector: devhost.ByEnvironment(env.Proto()),
	}, pkg.String())
	if err != nil {
		if errors.Is(err, ErrNoRegistry) {
			return nil, fnerrors.UsageError(
				fmt.Sprintf("Run `ns prepare local --env=%s` to set it up.", env.Proto().GetName()),
				"No registry configured in the environment %q.", env.Proto().GetName())
		}
		return nil, err
	}
	return allocated, nil
}

func RawAllocateName(ctx context.Context, ck *devhost.ConfigKey, repo string) (compute.Computable[oci.AllocatedName], error) {
	registry, err := GetRegistryFromConfig(ctx, ck)
	if err != nil {
		return nil, err
	}

	if registry == nil {
		return nil, ErrNoRegistry
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

func (r precomputedTag) ImageRef() string { return r.tag.ImageRef() }
