// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnany

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func Marshal(pkg schema.PackageName, msg proto.Message) (*anypb.Any, error) {
	typename := string(msg.ProtoReflect().Descriptor().FullName())

	msgBytes, err := proto.Marshal(msg)
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
	return fmt.Sprintf("%s%s/%s", protos.TypeUrlBaseSlash, pkg, typename)
}
