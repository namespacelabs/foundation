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
	"strings"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
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
		opt := prototext.UnmarshalOptions{
			Resolver: configMessageLookup{},
		}

		if err := opt.Unmarshal(devHostBytes, root.LoadedDevHost); err != nil {
			return fnerrors.BadInputError("Failed to parse %q. If you changed it manually, try to undo your changes. Saw: %w", DevHostFilename, err)
		}
	}

	for _, entry := range root.LoadedDevHost.Configure {
		if err := validate(entry.Configuration); err != nil {
			return err
		}

		for _, y := range entry.PlatformConfiguration {
			if err := validate(y.Configuration); err != nil {
				return err
			}
		}
	}

	for _, x := range root.LoadedDevHost.ConfigurePlatform {
		if err := validate(x.Configuration); err != nil {
			return err
		}
	}

	if err := validate(root.LoadedDevHost.ConfigureTools); err != nil {
		return err
	}

	for _, env := range root.Workspace().Proto().EnvSpec {
		if !HasRuntime(env.Runtime) {
			return fnerrors.InternalError("%s is not a supported runtime type", env.Runtime)
		}
	}

	return nil
}

type configMessageLookup struct{}

var _ protoregistry.MessageTypeResolver = configMessageLookup{}
var _ protoregistry.ExtensionTypeResolver = configMessageLookup{}

func (configMessageLookup) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	mt := planning.LookupConfigMessage(message)
	if mt == nil {
		return nil, fnerrors.BadInputError("%s: no such configuration message", message)
	}

	return mt, nil
}

func (cl configMessageLookup) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	if i := strings.LastIndexByte(url, '/'); i >= 0 {
		return cl.FindMessageByName(protoreflect.FullName(url[i+len("/"):]))
	}

	return nil, fnerrors.BadInputError("%s: no such configuration message url", url)
}

func (configMessageLookup) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	return protoregistry.GlobalTypes.FindExtensionByName(field)
}

func (configMessageLookup) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	return protoregistry.GlobalTypes.FindExtensionByNumber(message, field)
}

func validate(messages []*anypb.Any) error {
	for _, msg := range messages {
		if !planning.IsValidConfigType(msg) {
			return fnerrors.InternalError("%s: unsupported configuration type", msg.TypeUrl)
		}
	}

	return nil
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
