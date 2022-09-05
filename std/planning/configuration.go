// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

type Configuration interface {
	Get(proto.Message) bool
	GetForPlatform(specs.Platform, proto.Message) bool
	Derive(func([]*anypb.Any) []*anypb.Any) Configuration

	// HashKey returns a digest of the configuration that is being used.
	HashKey() string

	// Returns true if there's no configuration backing this Configuration
	// instance (i.e. no configuration was set).
	IsEmpty() bool

	// When the configuration is loaded pinned to an environment, returns the
	// environment name. Else, the return value is undefined.
	EnvKey() string

	getMultiple(string) []*anypb.Any
}

func GetMultiple[V proto.Message](config Configuration) ([]V, error) {
	msgs := config.getMultiple(protos.TypeUrl(protos.NewFromType[V]()))

	var result []V
	for _, msg := range msgs {
		v, err := msg.UnmarshalNew()
		if err != nil {
			return nil, err
		}
		result = append(result, v.(V))
	}

	return result, nil
}

func MakeConfigurationCompat(errorloc fnerrors.Location, ws *schema.Workspace, devHost *schema.DevHost, env *schema.Environment) (Configuration, error) {
	var base []*anypb.Any
	for _, spec := range ws.EnvSpec {
		if spec.Name == env.Name {
			base = spec.Configuration
		}
	}

	return makeConfigurationCompat(errorloc, base, devHost, env)
}

func makeConfigurationCompat(errorloc fnerrors.Location, base []*anypb.Any, devHost *schema.DevHost, env *schema.Environment) (Configuration, error) {
	rest := selectByEnv(devHost, env)
	merged := append(base, rest...)

	var parsed []*anypb.Any
	for _, m := range merged {
		if p, ok := configProviders[m.TypeUrl]; ok {
			messages, err := p(m)
			if err != nil {
				return nil, fnerrors.Wrapf(errorloc, err, "%s", m.TypeUrl)
			}

			for _, msg := range messages {
				any, err := anypb.New(msg)
				if err != nil {
					return nil, fnerrors.InternalError("%s: failed to serialize message: %w", m.TypeUrl, err)
				}

				parsed = append(parsed, any)
			}
		} else {
			parsed = append(parsed, m)
		}
	}

	return MakeConfigurationWith(env.Name, parsed, devHost.ConfigurePlatform), nil
}

func MakeConfigurationWith(description string, merged []*anypb.Any, platconfig []*schema.DevHost_ConfigurePlatform) Configuration {
	return config{description, merged, platconfig}
}

type ConfigurationSlice []*anypb.Any

type config struct {
	key        string
	merged     ConfigurationSlice
	platconfig []*schema.DevHost_ConfigurePlatform
}

func (cfg config) Get(msg proto.Message) bool {
	return checkGet(cfg.merged, msg)
}

func (cfg config) getMultiple(typeUrl string) []*anypb.Any {
	var response []*anypb.Any
	for _, m := range cfg.merged {
		if m.TypeUrl == typeUrl {
			response = append(response, m)
		}
	}
	return response
}

func checkGet(merged []*anypb.Any, msg proto.Message) bool {
	for _, conf := range merged {
		if conf.MessageIs(msg) {
			// XXX we're swallowing errors here.
			if conf.UnmarshalTo(msg) == nil {
				return true
			}
		}
	}

	return false
}

func (cfg config) GetForPlatform(target specs.Platform, msg proto.Message) bool {
	for _, p := range cfg.platconfig {
		if platformMatches(p, target) {
			if checkGet(p.Configuration, msg) {
				return true
			}

			break
		}
	}

	return cfg.Get(msg)
}

func (cfg config) HashKey() string {
	d, err := schema.DigestOf(cfg.merged)
	if err != nil {
		panic(err)
	}
	return d.String()
}

func (cfg config) IsEmpty() bool {
	return len(cfg.merged) == 0 && len(cfg.platconfig) == 0
}

func (cfg config) EnvKey() string {
	return cfg.key
}

func (cfg config) Derive(f func([]*anypb.Any) []*anypb.Any) Configuration {
	return config{
		key:        cfg.key,
		merged:     f(cfg.merged),
		platconfig: cfg.platconfig,
	}
}

func selectByEnv(devHost *schema.DevHost, env *schema.Environment) ConfigurationSlice {
	var slice ConfigurationSlice

	for _, cfg := range devHost.GetConfigure() {
		if cfg.Purpose != 0 && cfg.Purpose != env.GetPurpose() {
			continue
		}
		if cfg.Runtime != "" && cfg.Runtime != env.GetRuntime() {
			continue
		}
		if cfg.Name != "" && cfg.Name != env.GetName() {
			continue
		}

		slice = append(slice, cfg.Configuration...)
	}

	return slice
}

func platformMatches(a *schema.DevHost_ConfigurePlatform, b specs.Platform) bool {
	if a.Architecture != "" && a.Architecture != b.Architecture {
		return false
	}
	if a.Os != "" && a.Os != b.OS {
		return false
	}
	if a.Variant != "" && a.Variant != b.Variant {
		return false
	}

	return true
}
