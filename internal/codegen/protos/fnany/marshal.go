// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnany

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	wsprotos "namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func Marshal(pkg schema.PackageName, msg protoreflect.ProtoMessage) (*anypb.Any, error) {
	typename := string(msg.ProtoReflect().Descriptor().FullName())

	msgBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
	if err != nil {
		return nil, fnerrors.InternalError("%s: %s: failed to marshal message: %w", pkg, typename, err)
	}

	return &anypb.Any{
		TypeUrl: TypeURL(pkg, msg),
		Value:   msgBytes,
	}, nil
}

func TypeURL(pkg schema.PackageName, msg proto.Message) string {
	typename := string(msg.ProtoReflect().Descriptor().FullName())
	return fmt.Sprintf("%s%s/%s", wsprotos.FoundationTypeUrlBaseSlash, pkg, typename)
}
