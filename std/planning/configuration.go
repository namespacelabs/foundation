// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/schema"
)

type Configuration interface {
	Get(proto.Message) bool
	HashKey() string
	IsEmpty() bool
	EnvKey() string
}

type ConfigurationCompat interface {
	DevHost() *schema.DevHost
	Environment() *schema.Environment
}

func MakeConfigurationCompat(compat ConfigurationCompat) Configuration {
	return config{compat.Environment().Name, selectByEnv(compat.DevHost(), compat.Environment())}
}

func MakeConfigurationWith(description string, merged []*anypb.Any) Configuration {
	return config{description, merged}
}

type config struct {
	key    string
	merged []*anypb.Any
}

func (cfg config) Get(msg proto.Message) bool {
	for _, conf := range cfg.merged {
		if conf.MessageIs(msg) {
			// XXX we're swallowing errors here.
			if conf.UnmarshalTo(msg) == nil {
				return true
			}
		}
	}

	return false
}

func (cfg config) HashKey() string {
	d, err := schema.DigestOf(cfg.merged)
	if err != nil {
		panic(err)
	}
	return d.String()
}

func (cfg config) IsEmpty() bool {
	return len(cfg.merged) == 0
}

func (cfg config) EnvKey() string {
	return cfg.key
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
