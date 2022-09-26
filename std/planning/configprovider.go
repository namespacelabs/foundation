// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

var configProviders = map[string]func(*anypb.Any) ([]proto.Message, error){}

func RegisterConfigurationProvider[V proto.Message](handle func(V) ([]proto.Message, error)) {
	configProviders[protos.TypeUrl[V]()] = func(input *anypb.Any) ([]proto.Message, error) {
		msg := protos.NewFromType[V]()
		if err := input.UnmarshalTo(msg); err != nil {
			return nil, err
		}

		return handle(msg)
	}

	registeredKnownTypes = append(registeredKnownTypes, internalConfigType{
		message:    protos.NewFromType[V](),
		stacktrace: stacktrace.New(),
	})
}
