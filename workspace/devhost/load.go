// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devhost

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const DevHostFilename = "devhost.textpb"

var HasRuntime func(string) bool

func HostOnlyFiles() []string { return []string{DevHostFilename} }

func Prepare(ctx context.Context, root *workspace.Root) error {
	root.LoadedDevHost = &schema.DevHost{} // Make sure we always have an instance of DevHost, even if empty.

	devHostBytes, err := fs.ReadFile(root.ReadWriteFS(), DevHostFilename)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	} else {
		if err := prototext.Unmarshal(devHostBytes, root.LoadedDevHost); err != nil {
			return fnerrors.BadInputError("Failed to parse %q. If you changed it manually, try to undo your changes.", DevHostFilename)
		}
	}

	for _, env := range root.Workspace().GetEnv() {
		if !HasRuntime(env.Runtime) {
			return fnerrors.InternalError("%s is not a supported runtime type", env.Runtime)
		}
	}

	return nil
}

type ConfSlice struct {
	merged [][]*anypb.Any
}

func (conf ConfSlice) Get(msg proto.Message) bool {
	for _, m := range conf.merged {
		for _, conf := range m {
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
			Configuration: p,
		})
	}
	return parsed
}

func (conf ConfSlice) Merged() []*anypb.Any {
	var merged []*anypb.Any
	for _, p := range conf.merged {
		merged = append(merged, p...)
	}
	return merged
}

func MakeConfiguration(messages ...proto.Message) (*schema.DevHost_ConfigureEnvironment, error) {
	c := &schema.DevHost_ConfigureEnvironment{}
	for _, msg := range messages {
		packed, err := anypb.New(msg)
		if err != nil {
			return nil, err
		}
		c.Configuration = append(c.Configuration, packed)
	}
	return c, nil
}

func Update(devHost *schema.DevHost, confs ...*schema.DevHost_ConfigureEnvironment) (*schema.DevHost, bool) {
	copy := protos.Clone(devHost)

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

func RewriteWith(ctx context.Context, fsys fnfs.ReadWriteFS, filename string, devhost *schema.DevHost) error {
	serialized, err := (prototext.MarshalOptions{Multiline: true}).Marshal(devhost)
	if err != nil {
		return err
	}

	if err := fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), fsys, filename, func(w io.Writer) error {
		_, err := w.Write(serialized)
		return err
	}); err != nil {
		return err
	}

	return nil
}

func CheckEmptyErr(h *schema.DevHost) error {
	if len(h.Configure) == 0 && len(h.ConfigurePlatform) == 0 && len(h.ConfigureTools) == 0 {
		return fnerrors.UsageError("Try running `ns prepare local`.", "The workspace hasn't been configured for development yet.")
	}

	return nil
}
