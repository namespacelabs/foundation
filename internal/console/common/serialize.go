// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package common

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func SerializeToBytes(msg interface{}) ([]byte, error) {
	if msg, ok := msg.(proto.Message); ok {
		return protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
	}

	return json.Marshal(msg)
}
