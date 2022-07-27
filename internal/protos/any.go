// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"log"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

func WrapAnysOrDie(srcs ...protoreflect.ProtoMessage) []*anypb.Any {
	var out []*anypb.Any

	for _, src := range srcs {
		any, err := anypb.New(src)
		if err != nil {
			log.Fatalf("Failed to wrap %s proto in an Any proto: %s", src.ProtoReflect().Descriptor().FullName(), err)
		}
		out = append(out, any)
	}

	return out
}
