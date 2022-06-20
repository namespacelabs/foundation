// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buf

import (
	"context"
	"encoding/json"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
)

type bufConfig struct {
	Version string          `json:"version"`
	Lint    *bufConfigBlock `json:"lint"`
}

type bufConfigBlock struct {
	Use    []string `json:"use"`
	Except []string `json:"except,omitempty"`
}

type GenerateTmpl struct {
	Version string       `json:"version"`
	Plugins []PluginTmpl `json:"plugins"`
}

type PluginTmpl struct {
	Name   string   `json:"name,omitempty"`
	Remote string   `json:"remote,omitempty"`
	Out    string   `json:"out"`
	Opt    []string `json:"opt"`
}

func Run(ctx context.Context, bufImage v1.Image, io rtypes.IO, wd string, mounts []*rtypes.LocalMapping, args []string) error {
	c := bufConfig{
		Version: "v1",
		Lint: &bufConfigBlock{
			Use:    []string{"DEFAULT"},
			Except: []string{"PACKAGE_VERSION_SUFFIX"},
		},
	}

	configBytes, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return tools.Run(ctx, rtypes.RunToolOpts{
		ImageName: "namespacelabs/tools/buf", // This is just for local tagging, we never upload these packages.
		IO:        io,
		Mounts:    mounts,
		// NoNetworking: true, // XXX security
		RunBinaryOpts: rtypes.RunBinaryOpts{
			Image:      bufImage,
			WorkingDir: wd,
			Command:    []string{"buf"},
			Args:       append([]string{"--config", string(configBytes)}, args...),
			Env:        []*schema.BinaryConfig_EnvEntry{{Name: "HOME", Value: "/tmp"}},
			RunAsUser:  true, // Files written by buf will then be owned by the calling user.
		}})
}
