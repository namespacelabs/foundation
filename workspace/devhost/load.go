// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devhost

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const DevHostFilename = "devhost.textpb"

var HasRuntime func(*schema.Workspace, *schema.Environment, *schema.DevHost) bool

func HostOnlyFiles() []string { return []string{DevHostFilename} }

func Prepare(ctx context.Context, root *workspace.Root) error {
	root.DevHost = &schema.DevHost{} // Make sure we always have an instance of DevHost, even if empty.

	devHostBytes, err := fs.ReadFile(root.FS(), DevHostFilename)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if err := prototext.Unmarshal(devHostBytes, root.DevHost); err != nil {
			return fnerrors.BadInputError("Failed to parse %q. If you changed it manually, try to undo your changes.", DevHostFilename)
		}
	}

	if root.Workspace.Env == nil {
		root.Workspace.Env = []*schema.Environment{
			{
				Name:    "dev",
				Runtime: "kubernetes", // XXX
				Purpose: schema.Environment_DEVELOPMENT,
			},
			{
				Name:    "prod",
				Runtime: "kubernetes",
				Purpose: schema.Environment_PRODUCTION,
			},
		}
	}

	for _, env := range root.Workspace.Env {
		if !HasRuntime(root.Workspace, env, root.DevHost) {
			return fnerrors.InternalError("%s is not a supported runtime type", env.Runtime)
		}
	}

	return nil
}

type ConfSlice struct {
	merged []*schema.DevHost_ConfigureEnvironment
}

type PlatformConfSlice struct {
	merged []*schema.DevHost_ConfigurePlatform
}

func (conf ConfSlice) Get(msg proto.Message) bool {
	for _, m := range conf.merged {
		for _, conf := range m.Configuration {
			if conf.MessageIs(msg) {
				// XXX we're swallowing errors here.
				if conf.UnmarshalTo(msg) == nil {
					return true
				}
			}
		}
	}

	return false
}

func (conf PlatformConfSlice) Get(msg proto.Message) bool {
	for _, m := range conf.merged {
		for _, conf := range m.Configuration {
			if conf.MessageIs(msg) {
				// XXX we're swallowing errors here.
				if conf.UnmarshalTo(msg) == nil {
					return true
				}
			}
		}
	}

	return false
}

func (conf ConfSlice) WithoutConstraints() []*schema.DevHost_ConfigureEnvironment {
	var parsed []*schema.DevHost_ConfigureEnvironment
	for _, p := range conf.merged {
		parsed = append(parsed, &schema.DevHost_ConfigureEnvironment{
			Configuration: p.Configuration,
		})
	}
	return parsed
}

func ConfigurationForEnv(env ops.Environment) ConfSlice {
	return ConfigurationForEnvParts(env.DevHost(), env.Proto())
}

func ConfigurationForEnvParts(devHost *schema.DevHost, env *schema.Environment) ConfSlice {
	var slice ConfSlice

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

		slice.merged = append(slice.merged, cfg)
	}

	return slice
}

func PlatformConf(devHost *schema.DevHost, platform specs.Platform) PlatformConfSlice {
	var slice PlatformConfSlice
	for _, cfg := range devHost.GetConfigurePlatform() {
		if cfg.Architecture != "" && cfg.Architecture != platform.Architecture {
			continue
		}
		if cfg.Os != "" && cfg.Os != platform.OS {
			continue
		}
		if cfg.Variant != "" && cfg.Variant != platform.Variant {
			continue
		}
		slice.merged = append(slice.merged, cfg)
	}
	return slice
}

func MakeConfiguration(msg proto.Message) (*schema.DevHost_ConfigureEnvironment, error) {
	c := &schema.DevHost_ConfigureEnvironment{}
	packed, err := anypb.New(msg)
	if err != nil {
		return nil, err
	}
	c.Configuration = append(c.Configuration, packed)
	return c, nil
}

func Update(root *workspace.Root, confs ...*schema.DevHost_ConfigureEnvironment) (*schema.DevHost, bool) {
	copy := proto.Clone(root.DevHost).(*schema.DevHost)

	var totalChangeCount int
	for _, conf := range confs {

		for _, newCfg := range conf.Configuration {
			var exists bool

			for _, existing := range copy.Configure {
				if existing.Name != conf.Name || existing.Purpose != conf.Purpose || existing.Runtime != conf.Runtime {
					continue
				}

				for k, existingMsg := range existing.Configuration {
					if existingMsg.TypeUrl != newCfg.TypeUrl {
						continue
					}

					exists = true
					if bytes.Equal(existingMsg.Value, newCfg.Value) {
						// XXX use proto equality.
					} else {
						existing.Configuration[k] = newCfg
						totalChangeCount++
					}
					break
				}
			}

			if exists {
				continue
			}

			copy.Configure = append(copy.Configure, &schema.DevHost_ConfigureEnvironment{
				Configuration: []*anypb.Any{newCfg},
				Name:          conf.Name,
				Purpose:       conf.Purpose,
				Runtime:       conf.Runtime,
			})

			totalChangeCount++
		}
	}

	return copy, totalChangeCount > 0
}

func RewriteWith(ctx context.Context, root *workspace.Root, devhost *schema.DevHost) error {
	serialized, err := (prototext.MarshalOptions{Multiline: true}).Marshal(devhost)
	if err != nil {
		return err
	}

	if err := fnfs.WriteWorkspaceFile(ctx, root.FS(), DevHostFilename, func(w io.Writer) error {
		_, err := w.Write(serialized)
		return err
	}); err != nil {
		return err
	}

	return nil
}
