// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
	"namespacelabs.dev/foundation/internal/protos"
)

type ConfigType[V proto.Message] struct{}

type internalConfigType struct {
	message    proto.Message
	stacktrace stacktrace.StackTrace
}

var (
	registeredKnownTypes []internalConfigType
	wellKnownTypes       []string
)

func DefineConfigType[V proto.Message]() ConfigType[V] {
	configType := ConfigType[V]{}
	registeredKnownTypes = append(registeredKnownTypes, internalConfigType{protos.NewFromType[V](), stacktrace.New()})
	return configType
}

func ValidateNoConfigTypeCollisions() {
	const urlPrefix = "type.googleapis.com/"

	seen := map[string]stacktrace.StackTrace{}
	for _, wkt := range registeredKnownTypes {
		// We can only access ProtoReflect() after the proto package init() methods have been called.
		name := string(wkt.message.ProtoReflect().Descriptor().FullName())

		if st, ok := seen[name]; ok {
			panic(name + ": registered multiple times: " + st[0].File() + " vs " + wkt.stacktrace[0].File())
		}

		seen[name] = wkt.stacktrace
	}

	var strs []string
	for key := range seen {
		strs = append(strs, urlPrefix+key)
	}

	slices.Sort(strs)
	wellKnownTypes = strs

	registeredKnownTypes = nil
}

func IsValidConfigType(msg *anypb.Any) bool {
	return slices.Contains(wellKnownTypes, msg.TypeUrl)
}

func (ConfigType[V]) CheckGet(cfg Configuration) (V, bool) {
	v := protos.NewFromType[V]()
	if cfg == nil {
		return v, false
	}
	return v, cfg.checkGetMessage(v)
}

func (ConfigType[V]) CheckGetForPlatform(cfg Configuration, target specs.Platform) (V, bool) {
	v := protos.NewFromType[V]()
	if cfg == nil {
		return v, false
	}
	return v, cfg.checkGetMessageForPlatform(target, v)
}
