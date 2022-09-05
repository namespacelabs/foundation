// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/schema"
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
}

func MakeConfigurationCompat(ws *schema.Workspace, devHost *schema.DevHost, env *schema.Environment) Configuration {
	return MakeConfigurationWith(env.Name, selectByEnv(devHost, env), devHost.ConfigurePlatform)
}

func MakeConfigurationWith(description string, merged []*anypb.Any, platconfig []*schema.DevHost_ConfigurePlatform) Configuration {
	return config{description, merged, platconfig}
}

type config struct {
	key        string
	merged     []*anypb.Any
	platconfig []*schema.DevHost_ConfigurePlatform
}

func (cfg config) Get(msg proto.Message) bool {
	return checkGet(cfg.merged, msg)
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

func selectByEnv(devHost *schema.DevHost, env *schema.Environment) []*anypb.Any {
	var slice []*anypb.Any

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
