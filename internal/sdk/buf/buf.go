// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buf

import (
	"context"
	"encoding/json"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
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

func BuildAndrun(ctx context.Context, env ops.Environment, root *workspace.Root, packages workspace.Packages, io rtypes.IO, args ...string) error {
	// Wait for `buf` build to complete.
	result, err := compute.Get(ctx, Image(ctx, env, packages))
	if err != nil {
		return err
	}

	return Run(ctx, result.Value.(v1.Image), io, "/workspace", []*rtypes.LocalMapping{{HostPath: root.Abs(), ContainerPath: "/workspace"}}, args)
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
			Env:        map[string]string{"HOME": "/tmp"},
			RunAsUser:  true, // Files written by buf will then be owned by the calling user.
		}})
}
