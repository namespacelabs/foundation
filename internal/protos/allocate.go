// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package protos

import (
	"context"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func AllocateFrom(ctx context.Context, pctx ParseContext, value map[string]any) (proto.Message, error) {
	typeUrl, ok := value["@type"].(string)
	if !ok {
		return nil, fnerrors.New("@type is missing")
	}

	if !strings.HasPrefix(typeUrl, TypeUrlPrefix) {
		return nil, fnerrors.New("@type is missing %q prefix", TypeUrlPrefix)
	}

	fullName := strings.TrimPrefix(typeUrl, TypeUrlPrefix)

	delete(value, "@type")

	desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(fullName))
	if err != nil {
		return nil, fnerrors.New("%s: failed to load descriptor: %w", fullName, err)
	}

	if msgDesc, ok := desc.(protoreflect.MessageDescriptor); ok {
		msg, err := AllocateWellKnownMessage(ctx, pctx, msgDesc, value)
		if err != nil {
			return nil, fnerrors.New("%s: failed to allocate message: %w", fullName, err)
		}
		return msg, nil
	} else {
		return nil, fnerrors.New("%s: can't use descriptor, not a message", fullName)
	}
}
