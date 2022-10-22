// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
