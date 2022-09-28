// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"log"
	"reflect"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewFromType[V proto.Message]() V {
	var m V
	return reflect.New(reflect.TypeOf(m).Elem()).Interface().(V)
}

func TypeUrl[V proto.Message]() string {
	m := NewFromType[V]()
	const urlPrefix = "type.googleapis.com/"
	return urlPrefix + string(m.ProtoReflect().Descriptor().FullName())
}

func WrapAnysOrDie(srcs ...protoreflect.ProtoMessage) []*anypb.Any {
	var out []*anypb.Any

	for _, src := range srcs {
		out = append(out, WrapAnyOrDie(src))
	}

	return out
}

func WrapAnyOrDie(src protoreflect.ProtoMessage) *anypb.Any {
	any, err := anypb.New(src)
	if err != nil {
		log.Fatalf("Failed to wrap %s proto in an Any proto: %s", src.ProtoReflect().Descriptor().FullName(), err)
	}
	return any
}
