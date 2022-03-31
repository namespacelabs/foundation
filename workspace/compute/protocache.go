// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"reflect"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/cache"
)

func RegisterProtoCacheable() {
	RegisterCacheable[proto.Message](protoCacheable{})
}

type protoCacheable struct{}

func (protoCacheable) ComputeDigest(ctx context.Context, v interface{}) (schema.Digest, error) {
	_, h, err := marshalProto(v.(proto.Message))
	if err != nil {
		return schema.Digest{}, err
	}

	return h, nil
}

func (protoCacheable) LoadCached(ctx context.Context, c cache.Cache, t CacheableInstance, h schema.Digest) (Result[proto.Message], error) {
	raw := t.NewInstance()
	msg, ok := raw.(proto.Message)
	if !ok {
		return Result[proto.Message]{}, fnerrors.InternalError("expected %q to be a ptr to a proto.Message", reflect.TypeOf(raw))
	}

	bytes, err := c.Bytes(ctx, h)
	if err != nil {
		return Result[proto.Message]{}, err
	}

	if err := proto.Unmarshal(bytes, msg); err != nil {
		return Result[proto.Message]{}, err
	}

	return Result[proto.Message]{
		Digest: h,
		Value:  msg,
	}, nil
}

func (protoCacheable) Cache(ctx context.Context, c cache.Cache, msg proto.Message) (schema.Digest, error) {
	bytes, h, err := marshalProto(msg)
	if err != nil {
		return schema.Digest{}, err
	}

	if err := c.WriteBytes(ctx, h, bytes); err != nil {
		return schema.Digest{}, err
	}

	return h, err
}

func marshalProto(msg proto.Message) ([]byte, schema.Digest, error) {
	bytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(msg)
	if err != nil {
		return nil, schema.Digest{}, err
	}

	h, err := cache.DigestBytes(bytes)
	return bytes, h, err
}
