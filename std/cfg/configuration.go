// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cfg

import (
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
)

type Configuration interface {
	Derive(string, func(ConfigurationSlice) ConfigurationSlice) Configuration

	// When the configuration is loaded pinned to an environment, returns the
	// environment name. Else, the return value is undefined. This value MUST
	// NOT be used as an authoritative cache key.
	EnvKey() string

	Workspace() Workspace

	checkGetMessage(proto.Message, string, []string) bool
	checkGetMessageForPlatform(specs.Platform, proto.Message, string, []string) bool
	fetchMultiple(string) []*anypb.Any
}

func GetMultiple[V proto.Message](config Configuration) ([]V, error) {
	msgs := config.fetchMultiple(protos.TypeUrl[V]())

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

func MakeConfigurationCompat(errorloc fnerrors.Location, ws Workspace, devHost *schema.DevHost, env *schema.Environment) (Configuration, error) {
	var base ConfigurationSlice
	for _, spec := range ws.Proto().EnvSpec {
		if spec.Name == env.Name {
			base.Configuration = spec.Configuration
			base.PlatformConfiguration = spec.PlatformConfiguration
		}
	}

	return makeConfigurationCompat(errorloc, ws, base, devHost, env)
}

func makeConfigurationCompat(errorloc fnerrors.Location, ws Workspace, base ConfigurationSlice, devHost *schema.DevHost, env *schema.Environment) (Configuration, error) {
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

	return MakeConfigurationWith(env.Name, ws, merged), nil
}

func applyProvider(errorloc fnerrors.Location, merged []*anypb.Any) ([]*anypb.Any, error) {
	var parsed []*anypb.Any
	for _, m := range merged {
		if p, ok := configProviders[m.TypeUrl]; ok {
			messages, err := p(m)
			if err != nil {
				return nil, fnerrors.NewWithLocation(errorloc, "%s: %w", m.TypeUrl, err)
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

func MakeConfigurationWith(description string, ws Workspace, merged ConfigurationSlice) Configuration {
	return config{description, ws, merged}
}

type ConfigurationSlice struct {
	Configuration         []*anypb.Any
	PlatformConfiguration []*schema.DevHost_ConfigurePlatform
}

type config struct {
	envKey    string
	workspace Workspace
	atoms     ConfigurationSlice
}

func (cfg config) Workspace() Workspace {
	return cfg.workspace
}

func (cfg config) checkGetMessage(msg proto.Message, name string, aliases []string) bool {
	return checkGet(cfg.atoms.Configuration, msg, name, aliases)
}

func (cfg config) fetchMultiple(typeUrl string) []*anypb.Any {
	var response []*anypb.Any
	for _, m := range cfg.atoms.Configuration {
		if m.TypeUrl == typeUrl {
			response = append(response, m)
		}
	}
	return response
}

func checkGet(merged []*anypb.Any, msg proto.Message, name string, aliases []string) bool {
	for _, conf := range merged {
		if messageIs(conf, name, aliases) {
			// XXX we're swallowing errors here.
			if proto.Unmarshal(conf.Value, msg) == nil {
				return true
			}
		}
	}

	return false
}

func messageIs(x *anypb.Any, name string, aliases []string) bool {
	url := x.GetTypeUrl()
	if matchTypeUrl(url, name) {
		return true
	}
	for _, alias := range aliases {
		if matchTypeUrl(url, alias) {
			return true
		}
	}
	return false
}

func matchTypeUrl(url, name string) bool {
	if !strings.HasSuffix(url, name) {
		return false
	}
	return len(url) == len(name) || url[len(url)-len(name)-1] == '/'
}

func (cfg config) checkGetMessageForPlatform(target specs.Platform, msg proto.Message, name string, aliases []string) bool {
	for _, p := range cfg.atoms.PlatformConfiguration {
		if platformMatches(p, target) {
			if checkGet(p.Configuration, msg, name, aliases) {
				return true
			}

			break
		}
	}

	return cfg.checkGetMessage(msg, name, aliases)
}

func (cfg config) EnvKey() string {
	return cfg.envKey
}

func (cfg config) Derive(envKey string, f func(ConfigurationSlice) ConfigurationSlice) Configuration {
	return config{
		envKey:    envKey,
		workspace: cfg.workspace,
		atoms:     f(cfg.atoms),
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
