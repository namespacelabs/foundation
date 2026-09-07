// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"
	"crypto/sha256"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/schema"
)

func RegisterByteDigesters() {
	RegisterDigester[[]byte](bytesDigester{})
	RegisterDigester[bytestream.ByteStream](byteStreamDigester{})
}

type bytesDigester struct{}

func (bytesDigester) ComputeDigest(_ context.Context, value []byte) (schema.Digest, error) {
	h := sha256.Sum256(value)
	return schema.Digest{Algorithm: "sha256", Hex: fmtHex(h[:])}, nil
}

type byteStreamDigester struct{}

func (byteStreamDigester) ComputeDigest(ctx context.Context, value bytestream.ByteStream) (schema.Digest, error) {
	return bytestream.Digest(ctx, value)
}

func fmtHex(value []byte) string {
	const digits = "0123456789abcdef"
	encoded := make([]byte, len(value)*2)
	for i, b := range value {
		encoded[i*2] = digits[b>>4]
		encoded[i*2+1] = digits[b&0xf]
	}
	return string(encoded)
}
