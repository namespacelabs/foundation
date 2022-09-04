// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devhost

import (
	"encoding/json"
	"fmt"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/schema"
)

type ConfigKey struct {
	DevHost  *schema.DevHost
	Selector Selector
}

func ConfigKeyFromEnvironment(env planning.Context) *ConfigKey {
	return &ConfigKey{
		DevHost:  env.DevHost(),
		Selector: ByEnvironment(env.Environment()),
	}
}

type Selector interface {
	Description() string
	HashKey() string
	Select(*schema.DevHost) ConfSlice
}

func Select(devHost *schema.DevHost, selector Selector) ConfSlice {
	return selector.Select(devHost)
}

func ByEnvironment(env *schema.Environment) Selector {
	return byEnv{env}
}

type byEnv struct{ env *schema.Environment }

func (e byEnv) Description() string { return e.env.Name }

func (e byEnv) HashKey() string {
	serialized, err := json.Marshal(e.env)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("env:%s", serialized)
}

func (e byEnv) Select(devHost *schema.DevHost) ConfSlice {
	var slice ConfSlice

	for _, cfg := range devHost.GetConfigure() {
		if cfg.Purpose != 0 && cfg.Purpose != e.env.GetPurpose() {
			continue
		}
		if cfg.Runtime != "" && cfg.Runtime != e.env.GetRuntime() {
			continue
		}
		if cfg.Name != "" && cfg.Name != e.env.GetName() {
			continue
		}

		slice.merged = append(slice.merged, cfg.Configuration)
	}

	return slice
}

func ByBuildPlatform(platform specs.Platform) Selector {
	return byBuildPlatform{platform}
}

type byBuildPlatform struct{ platform specs.Platform }

func (b byBuildPlatform) Description() string { return FormatPlatform(b.platform) }
func (b byBuildPlatform) HashKey() string     { return FormatPlatform(b.platform) }

func (b byBuildPlatform) Select(devHost *schema.DevHost) ConfSlice {
	var slice ConfSlice
	for _, cfg := range devHost.GetConfigurePlatform() {
		if cfg.Architecture != "" && cfg.Architecture != b.platform.Architecture {
			continue
		}
		if cfg.Os != "" && cfg.Os != b.platform.OS {
			continue
		}
		if cfg.Variant != "" && cfg.Variant != b.platform.Variant {
			continue
		}
		slice.merged = append(slice.merged, cfg.Configuration)
	}
	return slice
}

func ForToolsRuntime() Selector { return toolsRuntime{} }

type toolsRuntime struct{}

func (toolsRuntime) Description() string { return "configure_tools" }
func (toolsRuntime) HashKey() string     { return "configure_tools" }

func (toolsRuntime) Select(devHost *schema.DevHost) ConfSlice {
	var slice ConfSlice
	slice.merged = append(slice.merged, devHost.GetConfigureTools())
	return slice
}
