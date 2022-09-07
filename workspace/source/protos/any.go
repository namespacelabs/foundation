// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	reflect "reflect"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewFromType[V proto.Message]() V {
	var m V
	return reflect.New(reflect.TypeOf(m).Elem()).Interface().(V)
}

func TypeUrl(msg proto.Message) string {
	packed, err := anypb.New(msg)
	if err != nil {
		panic(err)
	}
	return packed.GetTypeUrl()
}
