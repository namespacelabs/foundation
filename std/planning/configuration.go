// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

type Configuration interface {
	Get(proto.Message) bool
	GetForPlatform(specs.Platform, proto.Message) bool
	Derive(string, func(ConfigurationSlice) ConfigurationSlice) Configuration

	// HashKey returns a digest of the configuration that is being used.
	HashKey() string

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
	var base ConfigurationSlice
	for _, spec := range ws.EnvSpec {
		if spec.Name == env.Name {
			base.Configuration = spec.Configuration
			base.PlatformConfiguration = spec.PlatformConfiguration
		}
	}

	return makeConfigurationCompat(errorloc, base, devHost, env)
}

func makeConfigurationCompat(errorloc fnerrors.Location, base ConfigurationSlice, devHost *schema.DevHost, env *schema.Environment) (Configuration, error) {
	rest := selectByEnv(devHost, env)
	rest.PlatformConfiguration = append(rest.PlatformConfiguration, devHost.ConfigurePlatform...)

	merged := ConfigurationSlice{
		Configuration:         append(slices.Clone(base.Configuration), rest.Configuration...),
		PlatformConfiguration: append(slices.Clone(base.PlatformConfiguration), rest.PlatformConfiguration...),
	}

	if p, err := applyProvider(errorloc, merged.Configuration); err == nil {
		merged.Configuration = p
	} else {
		return nil, err
	}

	for _, plat := range merged.PlatformConfiguration {
		if p, err := applyProvider(errorloc, plat.Configuration); err == nil {
			plat.Configuration = p
		} else {
			return nil, err
		}
	}

	return MakeConfigurationWith(env.Name, merged), nil
}

func applyProvider(errorloc fnerrors.Location, merged []*anypb.Any) ([]*anypb.Any, error) {
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
	return parsed, nil
}

func MakeConfigurationWith(description string, merged ConfigurationSlice) Configuration {
	return config{description, merged}
}

type ConfigurationSlice struct {
	Configuration         []*anypb.Any
	PlatformConfiguration []*schema.DevHost_ConfigurePlatform
}

type config struct {
	envKey string
	atoms  ConfigurationSlice
}

func (cfg config) Get(msg proto.Message) bool {
	return checkGet(cfg.atoms.Configuration, msg)
}

func (cfg config) getMultiple(typeUrl string) []*anypb.Any {
	var response []*anypb.Any
	for _, m := range cfg.atoms.Configuration {
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
	for _, p := range cfg.atoms.PlatformConfiguration {
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
	d, err := schema.DigestOf(cfg.atoms)
	if err != nil {
		panic(err)
	}
	return d.String()
}

func (cfg config) EnvKey() string {
	return cfg.envKey
}

func (cfg config) Derive(envKey string, f func(ConfigurationSlice) ConfigurationSlice) Configuration {
	return config{
		envKey: envKey,
		atoms:  f(cfg.atoms),
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

		slice.Configuration = append(slice.Configuration, cfg.Configuration...)
		slice.PlatformConfiguration = append(slice.PlatformConfiguration, cfg.PlatformConfiguration...)
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
