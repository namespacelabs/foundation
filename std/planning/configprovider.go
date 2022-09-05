// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var configProviders = map[string]func(*anypb.Any) ([]proto.Message, error){}

func RegisterConfigProvider(msg proto.Message, handle func(*anypb.Any) ([]proto.Message, error)) {
	any, err := anypb.New(msg)
	if err != nil {
		panic(err)
	}

	configProviders[any.TypeUrl] = handle
}
