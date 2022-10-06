// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
	"namespacelabs.dev/foundation/internal/protos"
)

const urlPrefix = "type.googleapis.com/"

type ConfigType[V proto.Message] struct {
	aliases []string
}

type internalConfigType struct {
	message    proto.Message
	stacktrace stacktrace.StackTrace
	aliases    []string
}

var (
	registeredKnownTypes []internalConfigType
	wellKnownTypes       map[string]protoreflect.MessageType
)

func DefineConfigType[V proto.Message](aliases ...string) ConfigType[V] {
	message := protos.NewFromType[V]()

	configType := ConfigType[V]{aliases: aliases}

	registeredKnownTypes = append(registeredKnownTypes, internalConfigType{
		message:    message,
		stacktrace: stacktrace.New(),
		aliases:    configType.aliases,
	})

	return configType
}

func LookupConfigMessage(name protoreflect.FullName) protoreflect.MessageType {
	return wellKnownTypes[string(name)]
}

func ValidateNoConfigTypeCollisions() {
	seen := map[string]stacktrace.StackTrace{}
	m := map[string]protoreflect.MessageType{}
	for _, wkt := range registeredKnownTypes {
		// We can only access ProtoReflect() after the proto package init() methods have been called.
		name := string(wkt.message.ProtoReflect().Descriptor().FullName())
		names := append([]string{name}, wkt.aliases...)

		for _, name := range names {
			if st, ok := seen[name]; ok {
				panic(name + ": registered multiple times: " + st[0].File() + " vs " + wkt.stacktrace[0].File())
			}

			seen[name] = wkt.stacktrace
			m[name] = wkt.message.ProtoReflect().Type()
		}
	}

	wellKnownTypes = m

	registeredKnownTypes = nil
}

func IsValidConfigType(msg *anypb.Any) bool {
	if strings.HasPrefix(msg.TypeUrl, urlPrefix) {
		_, ok := wellKnownTypes[strings.TrimPrefix(msg.TypeUrl, urlPrefix)]
		return ok
	}

	return false
}

func (ct ConfigType[V]) CheckGet(cfg Configuration) (V, bool) {
	m := protos.NewFromType[V]()
	if cfg == nil {
		return m, false
	}
	name := string(m.ProtoReflect().Descriptor().FullName())
	return m, cfg.checkGetMessage(m, name, ct.aliases)
}

func (ct ConfigType[V]) CheckGetForPlatform(cfg Configuration, target specs.Platform) (V, bool) {
	m := protos.NewFromType[V]()
	if cfg == nil {
		return m, false
	}
	name := string(m.ProtoReflect().Descriptor().FullName())
	return m, cfg.checkGetMessageForPlatform(target, m, name, ct.aliases)
}
