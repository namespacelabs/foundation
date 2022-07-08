// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncue

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (v *CueV) DecodeToProtoMessage(msg proto.Message) error {
	b, err := v.MarshalJSON()
	if err != nil {
		return err
	}

	return (protojson.UnmarshalOptions{}).Unmarshal(b, msg)
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
