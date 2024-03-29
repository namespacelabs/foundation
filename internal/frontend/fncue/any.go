// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncue

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"namespacelabs.dev/foundation/internal/protos"
)

func (v *CueV) DecodeToProtoMessage(msg proto.Message) error {
	b, err := v.Val.MarshalJSON()
	if err != nil {
		return err
	}

	return (protojson.UnmarshalOptions{}).Unmarshal(b, msg)
}

func DecodeToTypedProtoMessage[V proto.Message](v *CueV) (V, error) {
	msg := protos.NewFromType[V]()
	if v != nil {
		if err := v.Val.Decode(&msg); err != nil {
			return msg, err
		}
	}

	return msg, nil
}

func (v *CueV) DecodeAs(msgtype protoreflect.MessageType) (proto.Message, error) {
	msg := msgtype.New().Interface()
	if v.Exists() {
		if err := v.DecodeToProtoMessage(msg); err != nil {
			return nil, err
		}
	}
	return msg, nil
}
