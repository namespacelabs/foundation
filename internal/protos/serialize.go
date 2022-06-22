// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type TextAndBinary struct {
	Text   []byte
	JSON   []byte
	Binary []byte
}

type SerializeOpts struct {
	JSON     bool
	Resolver interface {
		protoregistry.ExtensionTypeResolver
		protoregistry.MessageTypeResolver
	}
}

func SerializeMultiple(msgs ...proto.Message) ([]TextAndBinary, error) {
	return SerializeOpts{}.Serialize(msgs...)
}

func (opts SerializeOpts) Serialize(msgs ...proto.Message) ([]TextAndBinary, error) {
	var res []TextAndBinary
	for _, m := range msgs {
		text, err := prototext.MarshalOptions{Multiline: true, Resolver: opts.Resolver}.Marshal(m)
		if err != nil {
			return nil, fnerrors.New("textproto serialized failed: %w", err)
		}

		binary, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
		if err != nil {
			return nil, fnerrors.New("proto serialized failed: %w", err)
		}

		tb := TextAndBinary{Text: text, Binary: binary}
		if opts.JSON {
			json, err := protojson.MarshalOptions{Multiline: true, Resolver: opts.Resolver}.Marshal(m)
			if err != nil {
				return nil, fnerrors.New("json serialized failed: %w", err)
			}
			tb.JSON = json
		}

		res = append(res, tb)
	}

	return res, nil
}
