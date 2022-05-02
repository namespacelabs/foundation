// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type TextAndBinary struct {
	Text   []byte
	Binary []byte
}

func SerializeMultiple(msgs ...proto.Message) ([]TextAndBinary, error) {
	var res []TextAndBinary
	for _, m := range msgs {
		text, err := prototext.MarshalOptions{Multiline: true}.Marshal(m)
		if err != nil {
			return nil, err
		}

		binary, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
		if err != nil {
			return nil, err
		}

		res = append(res, TextAndBinary{text, binary})
	}

	return res, nil
}
