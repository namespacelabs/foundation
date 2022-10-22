// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package protos

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
)

type AnyResolver interface {
	protoregistry.ExtensionTypeResolver
	protoregistry.MessageTypeResolver
	FindEnumByName(enum protoreflect.FullName) (protoreflect.EnumType, error)
}

func AsResolver(pr *protoregistry.Files) (AnyResolver, error) {
	ptypes := &protoregistry.Types{}

	var errs []error
	pr.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Extensions().Len(); i++ {
			if err := ptypes.RegisterExtension(dynamicpb.NewExtensionType(fd.Extensions().Get(i))); err != nil {
				errs = append(errs, err)
			}
		}

		for i := 0; i < fd.Enums().Len(); i++ {
			if err := ptypes.RegisterEnum(dynamicpb.NewEnumType(fd.Enums().Get(i))); err != nil {
				errs = append(errs, err)
			}
		}

		for i := 0; i < fd.Messages().Len(); i++ {
			if err := ptypes.RegisterMessage(dynamicpb.NewMessageType(fd.Messages().Get(i))); err != nil {
				errs = append(errs, err)
			}
		}

		return true
	})

	return ptypes, multierr.New(errs...)
}
