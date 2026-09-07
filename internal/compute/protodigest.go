// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"
	"crypto/sha256"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
)

func RegisterProtoDigester() {
	RegisterDigester[proto.Message](protoDigester{})
}

type protoDigester struct{}

func (protoDigester) ComputeDigest(_ context.Context, msg proto.Message) (schema.Digest, error) {
	contents, err := (proto.MarshalOptions{Deterministic: true}).Marshal(msg)
	if err != nil {
		return schema.Digest{}, err
	}
	h := sha256.Sum256(contents)
	return schema.Digest{Algorithm: "sha256", Hex: fmtHex(h[:])}, nil
}
