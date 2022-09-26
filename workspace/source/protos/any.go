// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	reflect "reflect"

	"google.golang.org/protobuf/proto"
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
