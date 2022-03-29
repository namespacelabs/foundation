// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"encoding/json"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
)

var (
	TaskOutputTextLog = Output("text.log", "text/plain")
)

type ProtoResolver interface {
	protoregistry.ExtensionTypeResolver
	protoregistry.MessageTypeResolver
}

func TryProtoAsJson(pr ProtoResolver, msg proto.Message, multiline bool) ([]byte, error) {
	// XXX Need to rethink how we handle serialized any protos.
	//
	// if pr != nil {
	// 	body, err := (protojson.MarshalOptions{
	// 		UseProtoNames: true,
	// 		Multiline:     multiline,
	// 		Resolver:      pr,
	// 	}).Marshal(msg)
	// 	if err == nil {
	// 		return body, nil
	// 	}
	// }

	return json.Marshal(msg)
}