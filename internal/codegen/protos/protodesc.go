// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"io/fs"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func LoadDescriptorByName(src *FileDescriptorSetAndDeps, name string) (*protoregistry.Files, protoreflect.Descriptor, error) {
	pd, err := protodesc.NewFiles(src.AsFileDescriptorSet())
	if err != nil {
		return nil, nil, err
	}

	desc, err := pd.FindDescriptorByName(protoreflect.FullName(name))
	return pd, desc, err
}

func LoadMessageByName(src *FileDescriptorSetAndDeps, name string) (*protoregistry.Files, protoreflect.MessageDescriptor, error) {
	pd, desc, err := LoadDescriptorByName(src, name)
	if err != nil {
		return nil, nil, err
	}

	msgdesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, nil, fnerrors.UserError(nil, "%s: expected a message type", name)
	}

	return pd, msgdesc, nil
}

func (opts ParseOpts) LoadMessageAtLocation(fsys fs.FS, loc Location, sources []string, name string) (protoreflect.MessageDescriptor, error) {
	parsed, err := opts.ParseAtLocation(fsys, loc, sources)
	if err != nil {
		return nil, fnerrors.BadInputError("failed to parse proto sources %v: %w", sources, err)
	}

	_, msgdesc, err := LoadMessageByName(parsed, name)
	if err != nil {
		return nil, fnerrors.BadInputError("%s: failed to load message: %w", name, err)
	}

	return msgdesc, nil
}
